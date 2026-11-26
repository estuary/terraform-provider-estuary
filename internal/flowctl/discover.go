package flowctl

import (
	"context"
	"path/filepath"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

// discoveredResources returns each binding resource of the discovered capture name, by target, none without it.
func discoveredResources(tree *catalogTree, name string) map[string]map[string]any {
	var resources map[string]map[string]any = map[string]map[string]any{}

	for _, binding := range catalog.Bindings(tree.capture(name)) {
		if target := catalog.BindingCollection(binding); target != "" {
			resources[target] = catalog.BindingResource(binding)
		}
	}

	return resources
}

// discover runs flowctl discover on the workspace spec file, on dataPlane, and reads the discovered catalog tree back.
// Without a data plane, flowctl discovers a live capture on its own and a new one on the default.
func (client *Client) discover(ctx context.Context, directory string, dataPlane string) (*catalogTree, error) {
	var arguments []string = withDataPlane([]string{"discover"}, "--data-plane", dataPlane)

	if _, err := client.execCommand(ctx, append(arguments, "--source", filepath.Join(directory, specFile))...); err != nil {
		return nil, err
	}

	return readCatalogTree(directory)
}

// DiscoverCapture discovers the capture on dataPlane, see discover, returning each binding resource by target.
func (client *Client) DiscoverCapture(ctx context.Context, name string, dataPlane string, spec catalog.CaptureSpec) (map[string]map[string]any, error) {
	var resources map[string]map[string]any

	files, err := captureFiles(name, spec)

	if err != nil {
		return nil, err
	}

	ctx, cancel := client.connectorContext(ctx, "Discovering capture", files)

	defer cancel()

	err = inWorkspace(files, func(directory string) error {
		tree, err := client.discover(ctx, directory, dataPlane)

		if err == nil {
			resources = discoveredResources(tree, name)
		}

		return err
	})

	return resources, err
}
