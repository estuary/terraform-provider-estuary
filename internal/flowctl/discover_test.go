package flowctl

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestDiscoveredStreams(test *testing.T) {
	var directory string = test.TempDir()

	var content []byte = spectest.Dedent([]byte(`
		captures:
			acmeCo/mongodb/source:
				endpoint:
					connector:
						image: ghcr.io/estuary/source-mongodb:v3
						config: {}
				bindings:
					- resource: { database: shop, collection: users }
						target: acmeCo/mongodb/users
					- resource: { database: shop, collection: events }
						target: acmeCo/mongodb/events
					- resource: { database: shop, collection: orphan }
						target: acmeCo/mongodb/orphan
		collections:
			acmeCo/mongodb/users:
				key: [/_id]
				readSchema: users.read.schema.yaml
			acmeCo/mongodb/events:
				key: [/id, /ts]
				schema:
					properties:
						id: { type: integer }
	`))

	var schema []byte = spectest.Dedent([]byte(`
		properties:
			_id: { type: string, format: objectid }
	`))

	if err := os.WriteFile(filepath.Join(directory, "users.read.schema.yaml"), schema, 0600); err != nil {
		test.Fatalf("writing schema file: %v", err)
	}

	streams, err := discoveredStreams(content, "acmeCo/mongodb/source", directory)

	if err != nil {
		test.Fatalf("discoveredStreams: %v", err)
	}

	expected := map[string]catalog.DiscoveredStreamInfo{
		"acmeCo/mongodb/users": {
			Resource:   map[string]any{"database": "shop", "collection": "users"},
			Keys:       []string{"/_id"},
			KeySchemas: map[string]any{"_id": map[string]any{"type": "string", "format": "objectid"}},
		},
		"acmeCo/mongodb/events": {
			Resource: map[string]any{"database": "shop", "collection": "events"},
			Keys:     []string{"/id", "/ts"},
			KeySchemas: map[string]any{
				"id": map[string]any{"type": "integer"},
				"ts": map[string]any{"type": "string"},
			},
		},
	}

	if !reflect.DeepEqual(streams, expected) {
		test.Errorf("streams = %+v, want %+v", streams, expected)
	}
}
