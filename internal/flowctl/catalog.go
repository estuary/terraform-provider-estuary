package flowctl

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

// pull pulls the live specs of names into directory, in flowctl's nested layout, and reads them back.
func (client *Client) pull(ctx context.Context, directory string, names ...string) (*catalogTree, error) {
	var arguments []string = []string{"catalog", "pull-specs"}

	for _, name := range names {
		arguments = append(arguments, flagValue("--name", name))
	}

	if _, err := client.execCommand(ctx, append(arguments, "--overwrite", "--target", filepath.Join(directory, specFile))...); err != nil {
		return nil, err
	}

	return readCatalogTree(directory)
}

// pulledSpec returns the spec of name with its config inlined, nil when nothing was pulled, as for a deleted entry.
// Other pulled entries, or a spec without bindings, fail rather than reading as a deletion.
func pulledSpec(tree *catalogTree, name string) (map[string]any, error) {
	spec, file, found := tree.entry(capturesKey, name)

	if !found {
		spec, file, found = tree.entry(materializationsKey, name)
	}

	if names := tree.names(); !found && len(names) > 0 {
		return nil, fmt.Errorf("pulling %s returned no capture or materialization with that name, only: %s", name, strings.Join(names, ", "))
	}

	if !found {
		return nil, nil
	}

	if _, isList := spec[catalog.BindingsKey].([]any); !isList {
		return nil, fmt.Errorf("the pulled spec of %s has no bindings list", name)
	}

	if err := tree.inlineConfigs(spec, file); err != nil {
		return nil, err
	}

	return spec, nil
}

// ReadSpec returns the live spec of a capture or materialization, secrets still encrypted, nil when it doesn't exist.
func (client *Client) ReadSpec(ctx context.Context, name string) (map[string]any, error) {
	var spec map[string]any

	err := inWorkspace(nil, func(directory string) error {
		tree, err := client.pull(ctx, directory, name)

		if err == nil {
			spec, err = pulledSpec(tree, name)
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
