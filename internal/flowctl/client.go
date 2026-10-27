package flowctl

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const commandTimeout time.Duration = 5 * time.Minute
const connectorTimeout time.Duration = 20 * time.Minute
const specFile string = "flow.yaml"

type Client struct {
	binaryPath string
	apiToken   string
	dataPlane  string
}

var _ catalog.Backend = (*Client)(nil)

// flagValue joins a flag and its value in one argument, so flowctl never reads a dash-leading value as a flag.
func flagValue(flag string, value string) string {
	return flag + "=" + value
}

// inWorkspace runs use on a temporary workspace holding files, then removes it.
func inWorkspace(files map[string][]byte, use func(directory string) error) error {
	directory, err := os.MkdirTemp("", "estuary-tf-*")

	if err != nil {
		return fmt.Errorf("creating workspace: %w", err)
	}

	defer os.RemoveAll(directory)

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), content, 0600); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}

	return use(directory)
}

// maskedContext returns ctx masking the token and ctx secrets in logs, along with those values.
func (client *Client) maskedContext(ctx context.Context) (context.Context, []string) {
	var secrets []string = catalog.Secrets(ctx)

	if client.apiToken != "" {
		secrets = append(secrets, client.apiToken)
	}

	if len(secrets) == 0 {
		return ctx, secrets
	}

	return tflog.MaskLogStrings(ctx, secretForms(secrets)...), secrets
}

// execCommand runs flowctl, masking the token and ctx secrets in logs and errors.
func (client *Client) execCommand(ctx context.Context, arguments ...string) ([]byte, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	ctx, secrets := client.maskedContext(ctx)

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, commandTimeout)

		defer cancel()
	}

	var command *exec.Cmd = exec.CommandContext(ctx, client.binaryPath, arguments...)

	command.Env = append(os.Environ(), "FLOW_AUTH_TOKEN="+client.apiToken)

	command.Stdout = &stdout

	command.Stderr = &stderr

	tflog.Debug(ctx, "Executing flowctl command", map[string]any{
		"binary":    client.binaryPath,
		"arguments": strings.Join(arguments, " "),
	})

	if err := command.Run(); err != nil {
		var output string = maskStrings(stderr.String(), secrets)
		return nil, fmt.Errorf("flowctl %s: %w\nstderr: %s", strings.Join(arguments, " "), err, output)
	}

	return stdout.Bytes(), nil
}

func (client *Client) withDataPlane(arguments []string, flag string) []string {
	if client.dataPlane == "" {
		return arguments
	}

	return append(arguments, flagValue(flag, client.dataPlane))
}

// runOnSpec runs flowctl on the workspace file passed with fileFlag and returns its resulting content.
func (client *Client) runOnSpec(ctx context.Context, directory string, file string, fileFlag string, arguments ...string) ([]byte, error) {
	var path string = filepath.Join(directory, file)

	if _, err := client.execCommand(ctx, append(arguments, fileFlag, path)...); err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", file, err)
	}

	return content, nil
}

// rewriteSpec replaces the workspace spec file content with what rewrite returns for it.
func rewriteSpec(directory string, rewrite func(content []byte) ([]byte, error)) error {
	var path string = filepath.Join(directory, specFile)

	content, err := os.ReadFile(path)

	if err != nil {
		return fmt.Errorf("reading %s: %w", specFile, err)
	}

	content, err = rewrite(content)

	if err != nil {
		return err
	}

	return os.WriteFile(path, content, 0600)
}

// connectorContext returns ctx bounded by the connector timeout, logging message with the masked spec of files.
func (client *Client) connectorContext(ctx context.Context, message string, files map[string][]byte) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(ctx, connectorTimeout)

	logContext, secrets := client.maskedContext(ctx)

	tflog.Debug(logContext, message, map[string]any{"spec": maskedSpec(files[specFile], secrets)})

	return ctx, cancel
}

// NewClient creates a flowctl Client, an empty dataPlane using the account default. Catalog names include the tenant.
func NewClient(binaryPath, apiToken, dataPlane string) *Client {
	return &Client{
		binaryPath: binaryPath,
		apiToken:   apiToken,
		dataPlane:  dataPlane,
	}
}
