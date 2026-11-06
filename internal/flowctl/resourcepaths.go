package flowctl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Workspace directories of a dry run: the private flowctl home holding the draft it selects, and the validated specs.
const (
	draftHomeDirectory string = "home"
	validatedDirectory string = "validated"
)

// draftEnvironment returns the environment giving flowctl a private home under directory. Draft commands act on the
// draft flowctl selects in its profile, so parallel dry runs, or the user's own flowctl, never share it.
func draftEnvironment(directory string) ([]string, error) {
	var home string = filepath.Join(directory, draftHomeDirectory)

	if err := os.Mkdir(home, 0700); err != nil {
		return nil, fmt.Errorf("creating the flowctl home of the dry run: %w", err)
	}

	return []string{"HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, ".config")}, nil
}

// validatedPaths returns the resource path of each binding of the validated materialization name, in order, failing
// when the validated spec doesn't hold count bindings each with a path.
func validatedPaths(tree *catalogTree, name string, count int) ([][]string, error) {
	var paths [][]string

	spec, _, found := tree.entry(materializationsKey, name)

	if !found {
		return nil, fmt.Errorf("the validated draft has no materialization %s", name)
	}

	var bindings []any = catalog.Bindings(spec)

	if len(bindings) != count {
		return nil, fmt.Errorf("the validated draft of %s has %d bindings, not the %d validated", name, len(bindings), count)
	}

	for index, binding := range bindings {
		var path []string = catalog.BindingResourcePath(binding)

		if len(path) == 0 {
			return nil, fmt.Errorf("binding %d of the validated draft of %s has no resource path", index, name)
		}

		paths = append(paths, path)
	}

	return paths, nil
}

// ResourcePaths validates the materialization in a private draft, see draftEnvironment, publishing nothing, and
// returns the resource path Estuary gives each binding, in order. The draft is deleted afterwards.
func (client *Client) ResourcePaths(ctx context.Context, name string, spec catalog.MaterializationSpec) ([][]string, error) {
	var paths [][]string

	document, err := utils.YAMLDocument(spec)

	if err != nil {
		return nil, fmt.Errorf("decoding materialization %s: %w", name, err)
	}

	files, err := materializationFiles(name, spec)

	if err != nil {
		return nil, err
	}

	ctx, cancel := client.connectorContext(ctx, "Validating materialization resource paths", files)

	defer cancel()

	err = inWorkspace(files, func(directory string) error {
		var target string = filepath.Join(directory, validatedDirectory, specFile)

		environment, err := draftEnvironment(directory)

		if err != nil {
			return err
		}

		if err := os.Mkdir(filepath.Join(directory, validatedDirectory), 0700); err != nil {
			return fmt.Errorf("creating the workspace of the validated draft: %w", err)
		}

		if _, err := client.execCommandIn(ctx, environment, "draft", "create"); err != nil {
			return err
		}

		defer func() {
			if _, err := client.execCommandIn(context.WithoutCancel(ctx), environment, "draft", "delete"); err != nil {
				tflog.Warn(ctx, "Deleting the draft of a dry run", map[string]any{"error": err.Error()})
			}
		}()

		for _, arguments := range [][]string{
			{"draft", "author", "--source", filepath.Join(directory, specFile)},
			{"draft", "test"},
			{"draft", "develop", "--target", target},
		} {
			if _, err := client.execCommandIn(ctx, environment, arguments...); err != nil {
				return err
			}
		}

		tree, err := readCatalogTree(filepath.Join(directory, validatedDirectory))

		if err == nil {
			paths, err = validatedPaths(tree, name, len(catalog.Bindings(document)))
		}

		return err
	})

	return paths, err
}
