package translator

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestListNodeKeepsTheOrderOfSets(test *testing.T) {
	var items []any = []any{"z", "b", "m", "a"}

	list, err := newListNode(reflect.TypeFor[string]())

	if err != nil {
		test.Fatal(err)
	}

	value, err := list.terraformValue(items, tftypes.Set{ElementType: tftypes.String})

	if err != nil {
		test.Fatal(err)
	}

	converted, err := list.specValue(value)

	if err != nil || !reflect.DeepEqual(converted, items) {
		test.Errorf("spec value = %v (%v), want the items in their order", converted, err)
	}
}
