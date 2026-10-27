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
