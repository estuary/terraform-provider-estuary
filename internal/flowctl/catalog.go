package flowctl

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"gopkg.in/yaml.v3"
)

type pulledFile struct {
	Captures         map[string]map[string]any `yaml:"captures"`
	Collections      map[string]map[string]any `yaml:"collections"`
	Materializations map[string]map[string]any `yaml:"materializations"`
}

// names returns the sorted names of every pulled entry.
func (pulled pulledFile) names() []string {
	var names []string = slices.Collect(maps.Keys(pulled.Captures))

	names = slices.AppendSeq(names, maps.Keys(pulled.Collections))
	names = slices.AppendSeq(names, maps.Keys(pulled.Materializations))

	slices.Sort(names)

	return names
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

// pulledSpec returns the spec of name in the pulled content, with its expectPubId, nil only when nothing was pulled,
// as flowctl pulls nothing for a deleted entry. Content naming other entries, or a spec without a bindings list, fails
// rather than reading as a deletion or as every binding removed.
func pulledSpec(content []byte, name string, directory string) (map[string]any, error) {
	var pulled pulledFile

	if err := yaml.Unmarshal(content, &pulled); err != nil {
		return nil, fmt.Errorf("parsing pulled spec: %w", err)
	}

	spec, found := pulled.Captures[name]

	if !found {
		spec, found = pulled.Materializations[name]
	}

	if names := pulled.names(); !found && len(names) > 0 {
		return nil, fmt.Errorf("pulling %s returned no capture or materialization with that name, only: %s", name, strings.Join(names, ", "))
	}

	if !found {
		return nil, nil
	}

	if _, isList := spec[catalog.BindingsKey].([]any); !isList {
		return nil, fmt.Errorf("the pulled spec of %s has no bindings list", name)
	}

	if err := inlineConfigs(spec, directory); err != nil {
		return nil, err
	}

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
