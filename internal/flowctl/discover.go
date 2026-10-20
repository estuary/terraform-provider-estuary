package flowctl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v3"
)

type discoveredCollection struct {
	Key        []string `yaml:"key"`
	Schema     any      `yaml:"schema"`
	ReadSchema any      `yaml:"readSchema"`
}

func discoveredStreams(content []byte, name string, directory string) (map[string]catalog.DiscoveredStreamInfo, error) {
	var discovered catalogFile

	if err := yaml.Unmarshal(content, &discovered); err != nil {
		return nil, fmt.Errorf("parsing discovered spec: %w", err)
	}

	var streams map[string]catalog.DiscoveredStreamInfo = map[string]catalog.DiscoveredStreamInfo{}

	for _, binding := range discovered.Captures[name].Bindings {
		target, _ := binding["target"].(string)

		collection, found := discovered.Collections[target]

		if !found {
			continue
		}

		streams[target] = catalog.DiscoveredStreamInfo{
			Resource:   utils.Lookup(binding, "resource"),
			Keys:       collection.Key,
			KeySchemas: keySchemas(collection, directory),
		}
	}

	return streams, nil
}

func keySchemas(collection discoveredCollection, directory string) map[string]any {
	var properties map[string]any = schemaProperties(collection.ReadSchema, directory)

	if properties == nil {
		properties = schemaProperties(collection.Schema, directory)
	}

	var schemas map[string]any = make(map[string]any, len(collection.Key))

	for _, key := range collection.Key {
		var field string = strings.TrimPrefix(key, "/")

		if schema, found := properties[field]; found {
			schemas[field] = schema

			continue
		}

		schemas[field] = map[string]any{"type": "string"}
	}

	return schemas
}

func schemaProperties(schema any, directory string) map[string]any {
	if path, isPath := schema.(string); isPath {
		content, err := os.ReadFile(filepath.Join(directory, path))

		if err != nil {
			return nil
		}

		if err := yaml.Unmarshal(content, &schema); err != nil {
			return nil
		}
	}

	object, _ := schema.(map[string]any)

	properties, _ := object["properties"].(map[string]any)

	return properties
}

// DiscoverCapture discovers the capture bindings, keyed by their target collection name.
func (client *Client) DiscoverCapture(ctx context.Context, name string, spec catalog.CaptureSpec) (map[string]catalog.DiscoveredStreamInfo, error) {
	files, err := captureFiles(name, spec)

	if err != nil {
		return nil, err
	}

	tflog.Debug(ctx, "Discovering capture", map[string]any{"spec": string(utils.MaskYAML(files[specFile]))})

	content, directory, err := client.runOnSpec(ctx, files, "--source", "discover", "--flat")

	if err != nil {
		return nil, err
	}

	defer os.RemoveAll(directory)

	return discoveredStreams(content, name, directory)
}
