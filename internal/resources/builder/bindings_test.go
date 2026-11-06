package builder

import (
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestPairSame(test *testing.T) {
	var withPath resourcePath = resourcePath{key: func(tftypes.Value, tftypes.Value) (string, string) { return "", "" }}

	expectations := map[string]struct {
		resourcePath resourcePath
		from         []bindingKey
		to           []bindingKey
		want         []int
	}{
		"pairs by collection and resource path":       {withPath, []bindingKey{{"acmeCo/orders", "b", "", ""}, {"acmeCo/orders", "a", "", ""}}, []bindingKey{{"acmeCo/orders", "a", "", ""}, {"acmeCo/orders", "b", "", ""}}, []int{1, 0}},
		"never pairs another resource":                {withPath, []bindingKey{{"acmeCo/orders", "c", "", ""}}, []bindingKey{{"acmeCo/orders", "a", "", ""}}, []int{-1}},
		"never pairs unknown resource paths":          {withPath, []bindingKey{{"acmeCo/orders", "", "", ""}}, []bindingKey{{"acmeCo/orders", "a", "", ""}}, []int{-1}},
		"pairs equal keys in order":                   {withPath, []bindingKey{{"acmeCo/orders", "a", "", ""}, {"acmeCo/orders", "a", "", ""}}, []bindingKey{{"acmeCo/orders", "a", "", ""}, {"acmeCo/orders", "a", "", ""}}, []int{0, 1}},
		"pairs by collection without resource paths":  {resourcePath{}, []bindingKey{{"acmeCo/orders", "", "", ""}}, []bindingKey{{"acmeCo/users", "", "", ""}, {"acmeCo/orders", "", "", ""}}, []int{1}},
		"never pairs bindings of unknown collections": {resourcePath{}, []bindingKey{{"", "", "", ""}}, []bindingKey{{"", "", "", ""}}, []int{-1}},
		"nothing to pair":                             {withPath, nil, nil, []int{}},
		"pairs by resource path alone":                {withPath, []bindingKey{{"acmeCo/orders", "a", "", "public\x00orders"}}, []bindingKey{{"acmeCo/orders_v2", "b", "", "public\x00orders"}}, []int{0}},
		"never pairs another resource path":           {withPath, []bindingKey{{"acmeCo/orders", "a", "", "public\x00orders"}}, []bindingKey{{"acmeCo/orders", "a", "", "public\x00orders_v2"}}, []int{-1}},
		"pairs without resource path by computed key": {withPath, []bindingKey{{"acmeCo/orders", "a", "", ""}}, []bindingKey{{"acmeCo/orders", "b", "", "public\x00b"}, {"acmeCo/orders", "a", "", "public\x00a"}}, []int{1}},
		"prefers the resource path to a computed key": {withPath, []bindingKey{{"acmeCo/orders", "a", "", "public\x00b"}, {"acmeCo/orders", "a", "", ""}}, []bindingKey{{"acmeCo/orders", "a", "", "public\x00a"}, {"acmeCo/orders", "b", "", "public\x00b"}}, []int{1, 0}},
	}

	for name, expected := range expectations {
		if got := expected.resourcePath.pairSame(expected.from, expected.to); !reflect.DeepEqual(got, expected.want) {
			test.Errorf("%s: pairSame = %v, want %v", name, got, expected.want)
		}
	}
}

func TestPairManaged(test *testing.T) {
	var withPath resourcePath = resourcePath{key: func(tftypes.Value, tftypes.Value) (string, string) { return "", "" }}

	expectations := map[string]struct {
		from []bindingKey
		to   []bindingKey
		want []int
	}{
		"pairs a shared collection by resource path": {[]bindingKey{{"acmeCo/orders", "b", "", ""}, {"acmeCo/orders", "a", "", ""}}, []bindingKey{{"acmeCo/orders", "a", "", ""}, {"acmeCo/orders", "b", "", ""}}, []int{1, 0}},
		"pairs a moved binding with the one left":    {[]bindingKey{{"acmeCo/orders", "a", "", ""}, {"acmeCo/orders", "c", "", ""}}, []bindingKey{{"acmeCo/orders", "b", "", ""}, {"acmeCo/orders", "a", "", ""}}, []int{1, 0}},
		"pairs unknown resource paths in order":      {[]bindingKey{{"acmeCo/orders", "", "", ""}, {"acmeCo/orders", "", "", ""}}, []bindingKey{{"acmeCo/orders", "a", "", ""}, {"acmeCo/orders", "b", "", ""}}, []int{0, 1}},
		"leaves extra bindings unpaired":             {[]bindingKey{{"acmeCo/orders", "a", "", ""}, {"acmeCo/orders", "b", "", ""}}, []bindingKey{{"acmeCo/orders", "a", "", ""}}, []int{0, -1}},
		"never pairs another collection":             {[]bindingKey{{"acmeCo/orders", "a", "", ""}}, []bindingKey{{"acmeCo/users", "a", "", ""}}, []int{-1}},
	}

	for name, expected := range expectations {
		if got := withPath.pairManaged(expected.from, expected.to); !reflect.DeepEqual(got, expected.want) {
			test.Errorf("%s: pairManaged = %v, want %v", name, got, expected.want)
		}
	}
}

func TestPairLive(test *testing.T) {
	var withPath resourcePath = resourcePath{key: func(tftypes.Value, tftypes.Value) (string, string) { return "", "" }}

	expectations := map[string]struct {
		planned []bindingKey
		prior   []bindingKey
		live    []bindingKey
		want    []int
	}{
		"pairs the same resource":                    {[]bindingKey{{"acmeCo/orders", "a", "", ""}}, nil, []bindingKey{{"acmeCo/orders", "a", "", ""}}, []int{0}},
		"pairs a moved binding with its live one":    {[]bindingKey{{"acmeCo/orders", "z", "", ""}}, []bindingKey{{"acmeCo/orders", "x", "", ""}}, []bindingKey{{"acmeCo/orders", "y", "", ""}, {"acmeCo/orders", "x", "", ""}}, []int{1}},
		"never pairs an unmanaged binding":           {[]bindingKey{{"acmeCo/orders", "a", "", ""}, {"acmeCo/orders", "n", "", ""}}, []bindingKey{{"acmeCo/orders", "a", "", ""}}, []bindingKey{{"acmeCo/orders", "a", "", ""}, {"acmeCo/orders", "u", "", ""}}, []int{0, -1}},
		"never pairs a binding changed outside":      {[]bindingKey{{"acmeCo/orders", "x", "", ""}}, []bindingKey{{"acmeCo/orders", "x", "", ""}}, []bindingKey{{"acmeCo/orders", "y", "", ""}}, []int{-1}},
		"prefers the same resource to a moved one":   {[]bindingKey{{"acmeCo/orders", "y", "", ""}}, []bindingKey{{"acmeCo/orders", "x", "", ""}}, []bindingKey{{"acmeCo/orders", "x", "", ""}, {"acmeCo/orders", "y", "", ""}}, []int{1}},
		"pairs unknown resource paths through state": {[]bindingKey{{"acmeCo/orders", "", "", ""}}, []bindingKey{{"acmeCo/orders", "x", "", ""}}, []bindingKey{{"acmeCo/orders", "u", "", ""}, {"acmeCo/orders", "x", "", ""}}, []int{1}},
	}

	for name, expected := range expectations {
		var priors []int = withPath.pairManaged(expected.planned, expected.prior)
		var priorLives []int = withPath.pairSame(expected.prior, expected.live)

		if got := withPath.pairLive(expected.planned, expected.live, priors, priorLives); !reflect.DeepEqual(got, expected.want) {
			test.Errorf("%s: pairLive = %v, want %v", name, got, expected.want)
		}
	}
}

func TestPublishedBindings(test *testing.T) {
	var discovered map[string]any = map[string]any{"resource": map[string]any{"table": "events"}, "target": "acmeCo/events", "disable": true}
	var liveUsers map[string]any = map[string]any{"resource": map[string]any{"table": "users"}, "target": "acmeCo/users"}
	var plannedUsers map[string]any = map[string]any{"resource": map[string]any{"table": "users"}, "target": "acmeCo/users", "backfill": 1}
	var plannedOrders map[string]any = map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"}
	var liveOrders map[string]any = map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders", "backfill": 2}
	var liveArchive map[string]any = map[string]any{"resource": map[string]any{"table": "orders_archive"}, "target": "acmeCo/orders", "backfill": 5}
	var plannedArchive map[string]any = map[string]any{"resource": map[string]any{"table": "orders_archive"}, "target": "acmeCo/orders"}
	var liveUnmanaged map[string]any = map[string]any{"resource": map[string]any{"table": "orders_dashboard"}, "target": "acmeCo/orders", "backfill": 7}
	var plannedMoved map[string]any = map[string]any{"resource": map[string]any{"table": "orders_v2"}, "target": "acmeCo/orders"}
	var plannedNew map[string]any = map[string]any{"resource": map[string]any{"table": "orders_new"}, "target": "acmeCo/orders"}

	expectations := map[string]struct {
		planned       []any
		previous      []any
		live          []any
		keepUnmanaged bool
		want          []publishedBinding
	}{
		"keeps unmanaged live bindings": {
			planned:       []any{plannedOrders},
			live:          []any{discovered},
			keepUnmanaged: true,
			want:          []publishedBinding{{discovered, -1, 0}, {plannedOrders, 0, -1}},
		},
		"overrides a live binding in place": {
			planned:       []any{plannedUsers},
			live:          []any{discovered, liveUsers},
			keepUnmanaged: true,
			want:          []publishedBinding{{discovered, -1, 0}, {plannedUsers, 0, 1}},
		},
		"drops a binding no longer planned": {
			previous:      []any{liveUsers},
			live:          []any{discovered, liveUsers},
			keepUnmanaged: true,
			want:          []publishedBinding{{discovered, -1, 0}},
		},
		"appends new bindings in plan order": {
			planned:       []any{plannedOrders, plannedUsers},
			keepUnmanaged: true,
			want:          []publishedBinding{{plannedOrders, 0, -1}, {plannedUsers, 1, -1}},
		},
		"keeps everything without a plan": {
			live:          []any{discovered, liveUsers},
			keepUnmanaged: true,
			want:          []publishedBinding{{discovered, -1, 0}, {liveUsers, -1, 1}},
		},
		"keeps every binding of a shared collection": {
			planned:       []any{plannedArchive, plannedOrders},
			previous:      []any{liveOrders, liveArchive},
			live:          []any{liveOrders, liveArchive},
			keepUnmanaged: true,
			want:          []publishedBinding{{plannedOrders, 1, 0}, {plannedArchive, 0, 1}},
		},
		"drops only the removed binding of a shared collection": {
			planned:       []any{plannedArchive},
			previous:      []any{liveOrders, liveArchive},
			live:          []any{liveOrders, liveArchive},
			keepUnmanaged: true,
			want:          []publishedBinding{{plannedArchive, 0, 1}},
		},
		"publishes only planned bindings without keeping unmanaged ones": {
			planned: []any{plannedArchive, plannedOrders},
			live:    []any{discovered, liveOrders, liveArchive},
			want:    []publishedBinding{{plannedArchive, 0, 2}, {plannedOrders, 1, 1}},
		},
		"replaces only the managed binding a planned one moves": {
			planned:       []any{plannedMoved},
			previous:      []any{liveOrders},
			live:          []any{liveUnmanaged, liveOrders},
			keepUnmanaged: true,
			want:          []publishedBinding{{liveUnmanaged, -1, 0}, {plannedMoved, 0, 1}},
		},
		"adds a new binding beside an unmanaged one of its collection": {
			planned:       []any{plannedOrders, plannedNew},
			previous:      []any{liveOrders},
			live:          []any{liveOrders, liveUnmanaged},
			keepUnmanaged: true,
			want:          []publishedBinding{{plannedOrders, 0, 0}, {liveUnmanaged, -1, 1}, {plannedNew, 1, -1}},
		},
		"nothing to publish": {keepUnmanaged: true},
	}

	var withPath resourcePath = resourcePath{key: func(tftypes.Value, tftypes.Value) (string, string) { return "", "" }}

	keys := func(bindings []any) []bindingKey {
		var keys []bindingKey

		for _, binding := range bindings {
			table, _ := catalog.BindingResource(binding)["table"].(string)

			keys = append(keys, bindingKey{collection: catalog.BindingCollection(binding), path: table})
		}

		return keys
	}

	for name, expected := range expectations {
		var plannedKeys []bindingKey = keys(expected.planned)
		var previousKeys []bindingKey = keys(expected.previous)
		var live livePairing = withPath.livePairing(plannedKeys, previousKeys, withPath.pairManaged(plannedKeys, previousKeys), entryBindings{keys: keys(expected.live)})

		got, err := live.publishedBindings(expected.planned, expected.live, expected.keepUnmanaged)

		if err != nil || !reflect.DeepEqual(got, expected.want) {
			test.Errorf("%s: publishedBindings = %v (%v), want %v", name, got, err, expected.want)
		}
	}

	if _, err := (livePairing{}).publishedBindings([]any{plannedOrders}, nil, true); err == nil {
		test.Error("publishedBindings of documents not matching the paired bindings returned no error")
	}
}

func testPairing(test *testing.T, resourcePath resourcePath, plan tftypes.Value, state tftypes.Value, readLive liveReader) bindingPairing {
	pairing, err := resourcePath.newPairing(plan, state, readLive)

	if err != nil {
		test.Fatal(err)
	}

	return pairing
}
