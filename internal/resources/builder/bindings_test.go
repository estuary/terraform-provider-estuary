package builder

import (
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

func TestMergeBindings(test *testing.T) {
	var discovered map[string]any = map[string]any{"resource": map[string]any{"collection": "events"}, "target": "acmeCo/events", "disable": true}
	var liveUsers map[string]any = map[string]any{"resource": map[string]any{"collection": "users"}, "target": "acmeCo/users"}
	var plannedUsers map[string]any = map[string]any{"resource": map[string]any{"collection": "users"}, "target": "acmeCo/users", "backfill": 1}
	var plannedOrders map[string]any = map[string]any{"resource": map[string]any{"collection": "orders"}, "target": "acmeCo/orders"}

	expectations := map[string]struct {
		planned  []any
		previous []any
		live     []any
		want     []any
	}{
		"keeps unmanaged live bindings": {
			planned: []any{plannedOrders},
			live:    []any{discovered},
			want:    []any{discovered, plannedOrders},
		},
		"overrides a live binding in place": {
			planned: []any{plannedUsers},
			live:    []any{discovered, liveUsers},
			want:    []any{discovered, plannedUsers},
		},
		"drops a binding no longer planned": {
			previous: []any{liveUsers},
			live:     []any{discovered, liveUsers},
			want:     []any{discovered},
		},
		"appends new bindings in plan order": {
			planned: []any{plannedOrders, plannedUsers},
			want:    []any{plannedOrders, plannedUsers},
		},
		"keeps everything without a plan": {
			live: []any{discovered, liveUsers},
			want: []any{discovered, liveUsers},
		},
		"nothing to merge": {},
	}

	for name, expected := range expectations {
		if got := mergeBindings(expected.planned, expected.previous, expected.live, catalog.BindingCollection); !reflect.DeepEqual(got, expected.want) {
			test.Errorf("%s: mergeBindings = %v, want %v", name, got, expected.want)
		}
	}
}
