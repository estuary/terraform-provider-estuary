package translator

import (
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
