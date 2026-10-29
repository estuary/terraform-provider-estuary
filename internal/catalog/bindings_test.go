package catalog

import (
	"testing"
)

func TestBindingCollection(test *testing.T) {
	expectations := map[string]struct {
		binding any
		want    string
	}{
		"capture target":                {map[string]any{"target": "acmeCo/users"}, "acmeCo/users"},
		"materialization source":        {map[string]any{"source": "acmeCo/users"}, "acmeCo/users"},
		"materialization source object": {map[string]any{"source": map[string]any{"name": "acmeCo/users", "notBefore": "2026-01-01T00:00:00Z"}}, "acmeCo/users"},
		"without collection":            {map[string]any{"resource": map[string]any{"table": "users"}}, ""},
		"not an object":                 {"acmeCo/users", ""},
	}

	for name, expected := range expectations {
		if got := BindingCollection(expected.binding); got != expected.want {
			test.Errorf("%s: BindingCollection = %q, want %q", name, got, expected.want)
		}
	}
}

func TestBindingsByCollection(test *testing.T) {
	var orders map[string]any = map[string]any{"target": "acmeCo/orders"}
	var users map[string]any = map[string]any{"source": "acmeCo/users"}

	var indexed map[string]map[string]any = BindingsByCollection(Bindings(map[string]any{BindingsKey: []any{orders, users}}))

	if len(indexed) != 2 || indexed["acmeCo/orders"]["target"] != "acmeCo/orders" || indexed["acmeCo/users"]["source"] != "acmeCo/users" {
		test.Errorf("BindingsByCollection = %v, want orders and users by collection", indexed)
	}

	if Bindings(map[string]any{}) != nil {
		test.Error("Bindings of a document without bindings is not nil")
	}
}

func TestKeepResourceMeta(test *testing.T) {
	var live map[string]any = map[string]any{"resource": map[string]any{"table": "orders", ResourceMetaKey: map[string]any{"path": []any{"public", "orders"}}}}

	expectations := map[string]struct {
		binding  map[string]any
		wantMeta bool
	}{
		"copied":           {map[string]any{"resource": map[string]any{"table": "orders"}}, true},
		"own kept":         {map[string]any{"resource": map[string]any{"table": "orders", ResourceMetaKey: "own"}}, true},
		"without resource": {map[string]any{"target": "acmeCo/orders"}, false},
	}

	for name, expected := range expectations {
		var own any = BindingResource(expected.binding)[ResourceMetaKey]

		KeepResourceMeta(live, expected.binding)

		meta, found := BindingResource(expected.binding)[ResourceMetaKey]

		if found != expected.wantMeta {
			test.Errorf("%s: resource meta found = %v, want %v", name, found, expected.wantMeta)
		}

		if own != nil && meta != own {
			test.Errorf("%s: resource meta = %v, want its own %v", name, meta, own)
		}
	}
}
