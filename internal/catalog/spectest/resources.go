package spectest

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// AssertMaterializationResources checks the bindings have exactly the want resources, in order.
func AssertMaterializationResources[R comparable](test testing.TB, bindings []materialization.Binding[R], want []R) {
	test.Helper()

	if len(bindings) != len(want) {
		test.Fatalf("got %d bindings, want %d", len(bindings), len(want))
	}

	for index, expected := range want {
		if bindings[index].Resource != expected {
			test.Errorf("binding %d resource = %+v, want %+v", index, bindings[index].Resource, expected)
		}
	}
}
