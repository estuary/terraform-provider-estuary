package flowctl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v3"
)

const dekafConfigFile string = "dekaf.config.yaml"

func (client *Client) publish(ctx context.Context, files map[string][]byte) error {
	directory, err := writeWorkspace(files)

	if err != nil {
		return err
	}

	defer os.RemoveAll(directory)

	tflog.Debug(ctx, "Publishing spec", map[string]any{"spec": string(utils.MaskYAML(files[specFile]))})

	var arguments []string = client.withDataPlane([]string{"catalog", "publish", "--source", filepath.Join(directory, specFile), "--auto-approve"}, "--init-data-plane")

	_, err = client.execCommand(ctx, fileSecrets(files), arguments...)

	return err
}

func captureFiles(name string, spec catalog.CaptureSpec) (map[string][]byte, error) {
	content, err := yaml.Marshal(map[string]any{"captures": map[string]any{name: spec}})

	if err != nil {
		return nil, fmt.Errorf("marshaling capture %s: %w", name, err)
	}

	return map[string][]byte{specFile: content}, nil
}

func materializationFiles(name string, spec catalog.MaterializationSpec) (map[string][]byte, error) {
	var catalogSpec map[string]any = map[string]any{"materializations": map[string]any{name: spec}}

	document, err := utils.YAMLDocument(catalogSpec)

	if err != nil {
		return nil, fmt.Errorf("decoding materialization %s: %w", name, err)
	}

	var dekaf map[string]any = utils.Lookup(document, "materializations", name, "endpoint", "dekaf")

	if dekaf == nil || dekaf["config"] == nil {
		content, err := yaml.Marshal(catalogSpec)

		if err != nil {
			return nil, fmt.Errorf("marshaling materialization %s: %w", name, err)
		}

		return map[string][]byte{specFile: content}, nil
	}

	config, err := yaml.Marshal(dekaf["config"])

	if err != nil {
		return nil, fmt.Errorf("marshaling Dekaf config of %s: %w", name, err)
	}

	dekaf["config"] = dekafConfigFile

	content, err := yaml.Marshal(document)

	if err != nil {
		return nil, fmt.Errorf("marshaling materialization %s: %w", name, err)
	}

	return map[string][]byte{specFile: content, dekafConfigFile: config}, nil
}

func fileSecrets(files map[string][]byte) []string {
	var secrets []string

	for _, content := range files {
		secrets = append(secrets, utils.SecretValues(content)...)
	}

	return secrets
}

// PublishCapture publishes the capture spec under its catalog name.
func (client *Client) PublishCapture(ctx context.Context, name string, spec catalog.CaptureSpec) error {
	files, err := captureFiles(name, spec)

	if err != nil {
		return err
	}

	return client.publish(ctx, files)
}

// PublishMaterialization publishes the materialization spec under its catalog name.
func (client *Client) PublishMaterialization(ctx context.Context, name string, spec catalog.MaterializationSpec) error {
	files, err := materializationFiles(name, spec)

	if err != nil {
		return err
	}

	return client.publish(ctx, files)
}
