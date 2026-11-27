package native

import (
	"slices"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
)

func TestResourcePath(test *testing.T) {
	var resources map[string]capture.ResourcePather = map[string]capture.ResourcePather{
		"resource":           Resource{Name: "tickets", Interval: "PT5M"},
		"scheduled resource": ScheduledResource{Resource: Resource{Name: "tickets"}, Schedule: "0 0 * * *"},
	}

	for name, resource := range resources {
		if path := resource.ResourcePath(); !slices.Equal(path, []string{"tickets"}) {
			test.Errorf("%s: resource path = %q, want [tickets]", name, path)
		}
	}
}
