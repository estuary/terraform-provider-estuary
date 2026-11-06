package flowctl

import (
	"os"
	"path/filepath"
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
	var tree *catalogTree = writeTree(test, map[string]string{
		"flow.yaml":                "import: [acmeCo/mongodb/flow.yaml]\n",
		"acmeCo/mongodb/flow.yaml": string(content),
	})

	var resources map[string]map[string]any = discoveredResources(tree, "acmeCo/mongodb/source")

	expected := map[string]map[string]any{
		"acmeCo/mongodb/users":  {"database": "shop", "collection": "users"},
		"acmeCo/mongodb/events": {"database": "shop", "collection": "events"},
	}

	if !reflect.DeepEqual(resources, expected) {
		test.Errorf("resources = %+v, want %+v", resources, expected)
	}

	if missing := discoveredResources(tree, "acmeCo/mongodb/missing"); len(missing) != 0 {
		test.Errorf("resources of a missing capture = %v, want none", missing)
	}

	var directory string = test.TempDir()

	if err := os.WriteFile(filepath.Join(directory, specFile), []byte("captures: [unclosed"), 0600); err != nil {
		test.Fatal(err)
	}

	if _, err := readCatalogTree(directory); err == nil {
		test.Error("readCatalogTree accepted an unparseable spec")
	}
}
