package flowctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"gopkg.in/yaml.v3"
)

const expectPubIDKey string = "expectPubId"

type catalogRow struct {
	CatalogName       string `json:"catalogName"`
	LegacyCatalogName string `json:"catalog_name"`
}

type pulledFile struct {
	Captures         map[string]map[string]any `yaml:"captures"`
	Materializations map[string]map[string]any `yaml:"materializations"`
}

// ReadCatalogEntry returns the named catalog entry, or nil when it doesn't exist.
func (client *Client) ReadCatalogEntry(ctx context.Context, name string) (*catalog.CatalogEntry, error) {
	var arguments []string = client.withDataPlane([]string{"catalog", "list", "--name", name, "-o", "json"}, "--data-plane-name")

	output, err := client.execCommand(ctx, nil, arguments...)

	if err != nil {
		return nil, err
	}

	return findCatalogEntry(output, name)
}

// ReadSpec returns the live spec of the named capture or materialization as published, its secrets still encrypted,
// without its publication ID, nil when it doesn't exist.
func (client *Client) ReadSpec(ctx context.Context, name string) (map[string]any, error) {
	content, directory, err := client.runOnSpec(ctx, nil, "--target", "catalog", "pull-specs", "--name", name, "--flat", "--overwrite")

	if err != nil {
		return nil, err
	}

	defer os.RemoveAll(directory)

	return pulledSpec(content, name, directory)
}

// DeleteCatalogEntry deletes the named capture or materialization.
func (client *Client) DeleteCatalogEntry(ctx context.Context, name string) error {
	var arguments []string = client.withDataPlane([]string{"catalog", "delete", "--name", name, "--dangerous-auto-approve"}, "--data-plane-name")

	_, err := client.execCommand(ctx, nil, arguments...)

	return err
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

// inlineConfigs replaces each endpoint config flowctl wrote to its own file, named in the spec, with the file content.
func inlineConfigs(spec map[string]any, directory string) error {
	for _, endpoint := range utils.Lookup(spec, "endpoint") {
		object, _ := endpoint.(map[string]any)
		path, isPath := object["config"].(string)

		if !isPath {
			continue
		}

		content, err := os.ReadFile(filepath.Join(directory, path))

		if err != nil {
			return fmt.Errorf("reading pulled config %s: %w", path, err)
		}

		var config any

		if err := yaml.Unmarshal(content, &config); err != nil {
			return fmt.Errorf("parsing pulled config %s: %w", path, err)
		}

		object["config"] = config
	}

	return nil
}

func findCatalogEntry(output []byte, name string) (*catalog.CatalogEntry, error) {
	var decoder *json.Decoder = json.NewDecoder(bytes.NewReader(output))

	for {
		var value json.RawMessage

		if err := decoder.Decode(&value); errors.Is(err, io.EOF) {
			return nil, nil
		} else if err != nil {
			return nil, fmt.Errorf("parsing catalog list output: %w", err)
		}

		var rows []catalogRow

		if bytes.HasPrefix(value, []byte("[")) {
			if err := json.Unmarshal(value, &rows); err != nil {
				return nil, fmt.Errorf("parsing catalog list output: %w", err)
			}
		} else {
			var row catalogRow

			if err := json.Unmarshal(value, &row); err != nil {
				return nil, fmt.Errorf("parsing catalog list output: %w", err)
			}

			rows = append(rows, row)
		}

		for _, row := range rows {
			if row.CatalogName == name || row.LegacyCatalogName == name {
				return &catalog.CatalogEntry{CatalogName: name}, nil
			}
		}
	}
}
