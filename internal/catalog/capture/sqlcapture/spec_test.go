package sqlcapture

import (
	"slices"
	"testing"
)

func TestResourcePath(test *testing.T) {
	var resource Resource = Resource{Namespace: "Public", Stream: "Orders", Mode: BackfillModePrecise}

	if path := resource.ResourcePath(); !slices.Equal(path, []string{"Public", "Orders"}) {
		test.Errorf("resource path = %q, want [Public Orders]", path)
	}
}
