package flowctl

import (
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestDiscoveredResources(test *testing.T) {
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
					- resource: { database: shop, collection: untargeted }
		collections:
			acmeCo/mongodb/users:
				key: [/_id]
	`))

	resources, err := discoveredResources(content, "acmeCo/mongodb/source")

	if err != nil {
		test.Fatalf("discoveredResources: %v", err)
	}

	expected := map[string]map[string]any{
		"acmeCo/mongodb/users":  {"database": "shop", "collection": "users"},
		"acmeCo/mongodb/events": {"database": "shop", "collection": "events"},
	}

	if !reflect.DeepEqual(resources, expected) {
		test.Errorf("resources = %+v, want %+v", resources, expected)
	}

	if _, err := discoveredResources([]byte("captures: [unclosed"), "acmeCo/mongodb/source"); err == nil {
		test.Error("discoveredResources accepted an unparseable spec")
	}
}
