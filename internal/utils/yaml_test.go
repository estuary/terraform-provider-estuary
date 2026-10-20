package utils

import (
	"reflect"
	"testing"
)

func TestLookup(test *testing.T) {
	var document map[string]any = map[string]any{
		"endpoint": map[string]any{
			"connector": map[string]any{"image": "ghcr.io/estuary/source-test:v1"},
		},
		"bindings": []any{},
	}

	expectations := map[string]struct {
		keys []string
		want map[string]any
	}{
		"nested object":  {[]string{"endpoint", "connector"}, map[string]any{"image": "ghcr.io/estuary/source-test:v1"}},
		"no keys":        {nil, document},
		"missing key":    {[]string{"endpoint", "dekaf"}, nil},
		"through a list": {[]string{"bindings", "resource"}, nil},
		"not an object":  {[]string{"endpoint", "connector", "image"}, nil},
	}

	for name, expected := range expectations {
		if got := Lookup(document, expected.keys...); !reflect.DeepEqual(got, expected.want) {
			test.Errorf("%s: Lookup(%v) = %v, want %v", name, expected.keys, got, expected.want)
		}
	}
}

func TestYAMLDocument(test *testing.T) {
	type resource struct {
		Table  string `yaml:"table"`
		Schema string `yaml:"schema,omitempty"`
	}

	document, err := YAMLDocument(map[string]any{"resource": resource{Table: "orders"}, "backfill": 2})

	if err != nil {
		test.Fatal(err)
	}

	var want map[string]any = map[string]any{"resource": map[string]any{"table": "orders"}, "backfill": 2}

	if !reflect.DeepEqual(document, want) {
		test.Errorf("YAMLDocument = %v, want %v", document, want)
	}

	if document, err := YAMLDocument([]string{"orders"}); err == nil || document != nil {
		test.Errorf("YAMLDocument = %v, %v, want an error for a value that is not an object", document, err)
	}
}
