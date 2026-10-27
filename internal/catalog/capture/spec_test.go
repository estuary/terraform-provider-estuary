package capture

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

type testResource struct {
	Schema string `yaml:"schema"`
	Table  string `yaml:"table"`
	Mode   string `yaml:"mode,omitempty"`
}

func (resource testResource) ResourcePath() []string {
	return []string{resource.Schema, resource.Table}
}

func TestResourcePaths(test *testing.T) {
	var spec Spec[struct{}, testResource] = Spec[struct{}, testResource]{
		Bindings: []Binding[testResource]{
			{Resource: testResource{Schema: "Shop", Table: "Orders", Mode: "Precise"}},
			{Resource: testResource{Schema: "shop", Table: "orders"}},
			{Resource: testResource{Schema: "a", Table: "b/c"}},
		},
	}

	var paths []string = spec.ResourcePaths()

	discovered, err := spec.ResourcePath(map[string]any{"schema": "Shop", "table": "Orders", "extra": true})

	if err != nil {
		test.Fatalf("ResourcePath: %v", err)
	}

	if discovered != paths[0] || discovered == paths[1] {
		test.Errorf("discovered path %q, want the one of %q only, ignoring unknown fields and keeping the case", discovered, paths[0])
	}

	slashed, err := spec.ResourcePath(map[string]any{"schema": "a/b", "table": "c"})

	if err != nil || slashed == paths[2] {
		test.Errorf("path of a/b.c = %q (%v), want it distinct from a.b/c", slashed, err)
	}
}

func TestSpecsAddNewBindings(test *testing.T) {
	expectations := map[string]struct {
		spec Spec[struct{}, testResource]
		want bool
	}{
		"adding new bindings":     {Spec[struct{}, testResource]{AutoDiscover: &AutoDiscover{AddNewBindings: true}}, true},
		"not adding new bindings": {Spec[struct{}, testResource]{AutoDiscover: &AutoDiscover{}}, false},
		"without auto discover":   {Spec[struct{}, testResource]{}, false},
	}

	for name, expected := range expectations {
		var adder catalog.NewBindingsAdder = expected.spec

		if got := adder.AddsNewBindings(); got != expected.want {
			test.Errorf("%s: AddsNewBindings = %v, want %v", name, got, expected.want)
		}
	}
}

func TestSpecsGenerateBindings(test *testing.T) {
	expectations := map[string]struct {
		spec Spec[struct{}, testResource]
		want bool
	}{
		"with auto discover":    {Spec[struct{}, testResource]{AutoDiscover: &AutoDiscover{}}, true},
		"without auto discover": {Spec[struct{}, testResource]{}, false},
	}

	for name, expected := range expectations {
		var generator catalog.BindingsGenerator = expected.spec

		if got := generator.GeneratesBindings(); got != expected.want {
			test.Errorf("%s: GeneratesBindings = %v, want %v", name, got, expected.want)
		}
	}
}
