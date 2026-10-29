package flowctl

import (
	"context"
	"fmt"
	"os"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"gopkg.in/yaml.v3"
)

const expectPubIDKey string = "expectPubId"

type pulledFile struct {
	Captures         map[string]map[string]any `yaml:"captures"`
	Materializations map[string]map[string]any `yaml:"materializations"`
}

// inlineConfigs replaces each endpoint config file reference flowctl wrote with its content, rejecting paths outside
// directory.
func inlineConfigs(spec map[string]any, directory string) error {
	root, err := os.OpenRoot(directory)

	if err != nil {
		return fmt.Errorf("opening workspace: %w", err)
	}

	defer root.Close()

	for _, endpoint := range utils.Lookup(spec, catalog.EndpointKey) {
		object, _ := endpoint.(map[string]any)
		path, isPath := object[catalog.ConfigKey].(string)

		if !isPath {
			continue
		}

		content, err := root.ReadFile(path)

		if err != nil {
			return fmt.Errorf("reading pulled config %s: %w", path, err)
		}

		var config any

		if err := yaml.Unmarshal(content, &config); err != nil {
			return fmt.Errorf("parsing pulled config %s: %w", path, err)
		}

		object[catalog.ConfigKey] = config
	}

	return nil
}

func pulledSpec(content []byte, name string, directory string) (map[string]any, error) {
	var pulled pulledFile

	if err := yaml.Unmarshal(content, &pulled); err != nil {
		return nil, fmt.Errorf("parsing pulled spec: %w", err)
	}

	spec, found := pulled.Captures[name]

	if !found {
		spec = pulled.Materializations[name]
	}

	if err := inlineConfigs(spec, directory); err != nil {
		return nil, err
	}

	delete(spec, expectPubIDKey)

	return spec, nil
}

// ReadSpec returns the live spec of a capture or materialization, secrets still encrypted, nil when it doesn't exist.
func (client *Client) ReadSpec(ctx context.Context, name string) (map[string]any, error) {
	var spec map[string]any

	err := inWorkspace(nil, func(directory string) error {
		content, err := client.runOnSpec(ctx, directory, specFile, "--target", "catalog", "pull-specs", flagValue("--name", name), "--flat", "--overwrite")

		if err == nil {
			spec, err = pulledSpec(content, name, directory)
		}

		return err
	})

	return spec, err
}

// DeleteCatalogEntry deletes a capture or materialization, whatever its data plane.
func (client *Client) DeleteCatalogEntry(ctx context.Context, name string) error {
	_, err := client.execCommand(ctx, "catalog", "delete", flagValue("--name", name), "--dangerous-auto-approve")

	return err
}
