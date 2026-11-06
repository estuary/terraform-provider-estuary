package translator

import (
	"math/big"
	"reflect"
	"testing"
)

func TestNewNodeRejectsUnsupportedTypes(test *testing.T) {
	for name, kind := range map[string]reflect.Type{
		"map with int keys": reflect.TypeFor[map[int]string](),
		"map of lists":      reflect.TypeFor[map[string][]string](),
		"map of structs": reflect.TypeFor[map[string]struct {
			Name string `yaml:"name"`
		}](),
		"key set with int keys": reflect.TypeFor[map[int]struct{}](),
		"nested list":           reflect.TypeFor[[][]string](),
		"interface":             reflect.TypeFor[any](),
		"struct with an any": reflect.TypeFor[struct {
			Value any `yaml:"value"`
		}](),
		"list of maps": reflect.TypeFor[[]map[string]string](),
		"struct with a channel": reflect.TypeFor[struct {
			Events chan int `yaml:"events"`
		}](),
	} {
		if _, err := newNode(kind); err == nil {
			test.Errorf("%s: newNode accepted %s", name, kind)
		}
	}
}

func TestDynamicNodeKeepsLargeIntegers(test *testing.T) {
	var large int = 1<<53 + 1

	value, err := dynamicNode{}.terraformValue(large, nil)

	if err != nil {
		test.Fatal(err)
	}

	var number big.Float

	if err := value.As(&number); err != nil {
		test.Fatal(err)
	}

	if converted, accuracy := number.Int64(); converted != int64(large) || accuracy != big.Exact {
		test.Errorf("converted = %d (%v), want %d exactly", converted, accuracy, large)
	}
}
