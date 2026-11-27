package sqlbatch

import (
	"slices"
	"testing"
)

func TestResourcePath(test *testing.T) {
	var resource Resource = Resource{Name: "orders_by_day", Schema: "shop", Table: "orders", Template: "SELECT 1"}

	if path := resource.ResourcePath(); !slices.Equal(path, []string{"orders_by_day"}) {
		test.Errorf("resource path = %q, want [orders_by_day]", path)
	}
}
