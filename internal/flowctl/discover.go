package flowctl

import (
	"context"
	"fmt"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v3"
)

type discoveredTask struct {
	Bindings []map[string]any `yaml:"bindings"`
}

type discoveredFile struct {
	Captures map[string]discoveredTask `yaml:"captures"`
}

func discoveredResources(content []byte, name string) (map[string]map[string]any, error) {
	var discovered discoveredFile

	if err := yaml.Unmarshal(content, &discovered); err != nil {
		return nil, fmt.Errorf("parsing discovered spec: %w", err)
	}

	var resources map[string]map[string]any = map[string]map[string]any{}

	for _, binding := range discovered.Captures[name].Bindings {
		if target, _ := binding["target"].(string); target != "" {
			resources[target] = utils.Lookup(binding, "resource")
		}
	}

	return resources, nil
}

// DiscoverCapture discovers the capture on the client data plane, returning each binding resource by target.
func (client *Client) DiscoverCapture(ctx context.Context, name string, spec catalog.CaptureSpec) (map[string]map[string]any, error) {
	var resources map[string]map[string]any

	files, err := captureFiles(name, spec)

	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, connectorTimeout)

	defer cancel()

	logContext, _ := client.maskedContext(ctx)

	tflog.Debug(logContext, "Discovering capture", map[string]any{"spec": string(files[specFile])})

	err = inWorkspace(files, func(directory string) error {
		content, err := client.runOnSpec(ctx, directory, "--source", client.withDataPlane([]string{"discover", "--flat"}, "--data-plane")...)

		if err == nil {
			resources, err = discoveredResources(content, name)
		}

		return err
	})

	return resources, err
}
