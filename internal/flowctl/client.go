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
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Client implements catalog.Backend by running the flowctl CLI.
type Client struct {
	binaryPath string
	apiToken   string
	dataPlane  string
}

// taskFile is a capture or materialization of a spec file flowctl writes.
type taskFile struct {
	Bindings []map[string]any `yaml:"bindings"`
}

// catalogFile is the layout of the spec file flowctl discovery writes, its bindings kept raw to retain every key.
type catalogFile struct {
	Captures         map[string]taskFile             `yaml:"captures"`
	Materializations map[string]taskFile             `yaml:"materializations"`
	Collections      map[string]discoveredCollection `yaml:"collections"`
}

// commandTimeout bounds flowctl commands whose context has no deadline.
const commandTimeout time.Duration = 5 * time.Minute

// specFile is the catalog spec file name written for every flowctl command.
const specFile string = "flow.yaml"

var _ catalog.Backend = (*Client)(nil)

// execCommand runs flowctl, masking the token and the given secrets in its logs and errors.
func (client *Client) execCommand(ctx context.Context, secrets []string, arguments ...string) ([]byte, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if client.apiToken != "" {
		secrets = append(secrets, client.apiToken)
	}

	if len(secrets) > 0 {
		ctx = tflog.MaskLogStrings(ctx, secrets...)
	}

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
		var output string = utils.MaskStrings(stderr.String(), secrets)
		return nil, fmt.Errorf("flowctl %s: %w\nstderr: %s", strings.Join(arguments, " "), err, output)
	}

	return stdout.Bytes(), nil
}

func (client *Client) withDataPlane(arguments []string, flag string) []string {
	if client.dataPlane == "" {
		return arguments
	}

	return append(arguments, flag, client.dataPlane)
}

func writeWorkspace(files map[string][]byte) (string, error) {
	directory, err := os.MkdirTemp("", "estuary-tf-*")

	if err != nil {
		return "", fmt.Errorf("creating workspace: %w", err)
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), content, 0600); err != nil {
			os.RemoveAll(directory)
			return "", fmt.Errorf("writing %s: %w", name, err)
		}
	}

	return directory, nil
}

// runOnSpec runs flowctl on a workspace holding files, passing the spec file path with specFlag, and returns
// the spec file flowctl leaves and the workspace directory, which the caller removes.
func (client *Client) runOnSpec(ctx context.Context, files map[string][]byte, specFlag string, arguments ...string) ([]byte, string, error) {
	directory, err := writeWorkspace(files)

	if err != nil {
		return nil, "", err
	}

	var path string = filepath.Join(directory, specFile)

	if _, err := client.execCommand(ctx, fileSecrets(files), append(arguments, specFlag, path)...); err != nil {
		os.RemoveAll(directory)
		return nil, "", err
	}

	content, err := os.ReadFile(path)

	if err != nil {
		os.RemoveAll(directory)
		return nil, "", fmt.Errorf("reading %s: %w", specFile, err)
	}

	return content, directory, nil
}

// NewClient creates a Client for the flowctl binary, an empty dataPlane using the account default.
// Catalog names are taken as given, already prefixed with the tenant.
func NewClient(binaryPath, apiToken, dataPlane string) *Client {
	return &Client{
		binaryPath: binaryPath,
		apiToken:   apiToken,
		dataPlane:  dataPlane,
	}
}
