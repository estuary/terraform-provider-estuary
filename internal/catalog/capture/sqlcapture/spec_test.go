package sqlcapture

import "testing"

func TestResourceRecommendedName(test *testing.T) {
	expectations := map[string]struct {
		resource Resource
		want     string
	}{
		"lowercases":                   {Resource{Namespace: "Public", Stream: "Orders"}, "public/orders"},
		"sanitizes invalid characters": {Resource{Namespace: "sales data", Stream: "orders$2024"}, "sales_data/orders_2024"},
		"keeps dashes and dots":        {Resource{Namespace: "my-db", Stream: "v1.orders"}, "my-db/v1.orders"},
		"rewrites dot path elements":   {Resource{Namespace: ".", Stream: ".."}, "_/__"},
	}

	for name, expected := range expectations {
		if got := expected.resource.RecommendedName(); got != expected.want {
			test.Errorf("%s: recommended name = %q, want %q", name, got, expected.want)
		}
	}
}
