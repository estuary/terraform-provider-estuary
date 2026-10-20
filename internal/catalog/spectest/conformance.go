package spectest

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

// SchemaFile is the connector schema snapshot every connector package keeps under testdata.
const SchemaFile string = "testdata/schema.json"

// AssertConformsToSchema checks the connector Config and Resource map exactly the properties of the
// connector schema snapshot, both ways. Legacy lists the paths of mapped fields the schema hides
// (e.g. "password" or "compute/aws_access_key_id"), which the connector still accepts.
func AssertConformsToSchema[C, R any](test *testing.T, legacy ...string) {
	test.Helper()

	var schema catalog.ConnectorSchema

	if err := json.Unmarshal(readFile(test, SchemaFile), &schema); err != nil {
		test.Fatalf("decoding %s: %v", SchemaFile, err)
	}

	sections := map[string]struct {
		mapped reflect.Type
		schema map[string]any
	}{
		"config": {
			mapped: reflect.TypeFor[C](),
			schema: schema.Endpoint,
		},
		"resource": {
			mapped: reflect.TypeFor[R](),
			schema: schema.Resource,
		},
	}

	for name, section := range sections {
		var differences []string = conformance("", section.mapped, section.schema, section.schema, legacy)

		if len(differences) > 0 {
			test.Errorf("%s differs from %s:\n%s", name, SchemaFile, joinLines(differences))
		}
	}
}

func conformance(path string, mapped reflect.Type, schema map[string]any, root map[string]any, legacy []string) []string {
	var differences []string

	var properties map[string]map[string]any = schemaProperties(schema, root)

	var fields map[string]reflect.Type = mappedFields(mapped)

	for _, key := range sortedKeys(toAny(properties)) {
		if _, found := fields[key]; !found {
			differences = append(differences, fmt.Sprintf("%s: in the schema but not mapped", path+key))
		}
	}

	for _, key := range sortedKeys(toAny(fields)) {
		property, found := properties[key]

		if !found {
			if !slices.Contains(legacy, path+key) {
				differences = append(differences, fmt.Sprintf("%s: mapped but not in the schema", path+key))
			}

			continue
		}

		var nested reflect.Type = structType(fields[key])

		if nested == nil {
			continue
		}

		if fields[key].Kind() == reflect.Slice {
			property, _ = property["items"].(map[string]any)
		}

		differences = append(differences, conformance(path+key+"/", nested, property, root, legacy)...)
	}

	return differences
}

func schemaProperties(schema map[string]any, root map[string]any) map[string]map[string]any {
	var properties map[string]map[string]any = map[string]map[string]any{}

	if reference, found := schema["$ref"].(string); found {
		schema = resolve(reference, root)
	}

	if object, found := schema["properties"].(map[string]any); found {
		for key, property := range object {
			nested, _ := property.(map[string]any)

			properties[key] = nested
		}
	}

	for _, keyword := range []string{"oneOf", "anyOf", "allOf"} {
		branches, _ := schema[keyword].([]any)

		for _, branch := range branches {
			nested, _ := branch.(map[string]any)

			for key, property := range schemaProperties(nested, root) {
				properties[key] = property
			}
		}
	}

	return properties
}

func resolve(reference string, root map[string]any) map[string]any {
	var node any = root

	for _, key := range strings.Split(strings.TrimPrefix(reference, "#/"), "/") {
		object, _ := node.(map[string]any)

		node = object[key]
	}

	resolved, _ := node.(map[string]any)

	return resolved
}

func mappedFields(mapped reflect.Type) map[string]reflect.Type {
	var fields map[string]reflect.Type = map[string]reflect.Type{}

	mapped = structType(mapped)

	if mapped == nil {
		return fields
	}

	for index := range mapped.NumField() {
		var field reflect.StructField = mapped.Field(index)

		var name string = strings.Split(field.Tag.Get("yaml"), ",")[0]

		if name != "" && name != "-" {
			fields[name] = field.Type
		}
	}

	return fields
}

func structType(mapped reflect.Type) reflect.Type {
	for mapped.Kind() == reflect.Pointer || mapped.Kind() == reflect.Slice {
		mapped = mapped.Elem()
	}

	if mapped.Kind() != reflect.Struct {
		return nil
	}

	return mapped
}

func toAny[V any](values map[string]V) map[string]any {
	var converted map[string]any = make(map[string]any, len(values))

	for key, value := range values {
		converted[key] = value
	}

	return converted
}
