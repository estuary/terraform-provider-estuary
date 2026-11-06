package spectest

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

// SchemaFile is the connector schema snapshot of each connector package.
const SchemaFile string = "testdata/schema.json"

// AssertConformsToSchema checks Config and Resource match the schema snapshot both ways, including secrets and enums.
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
	var fields map[string]reflect.StructField = mappedFields(mapped)

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

		var nested reflect.Type = structType(fields[key].Type)

		if nested == nil {
			differences = append(differences, scalarConformance(path+key, fields[key], property, schema, root)...)

			if slices.Contains(schemaRequired(schema, root), key) && dropsValidZero(fields[key]) {
				differences = append(differences, fmt.Sprintf("%s: required by the schema but omitted when zero", path+key))
			}

			continue
		}

		if fields[key].Type.Kind() == reflect.Slice {
			property, _ = property["items"].(map[string]any)
		}

		differences = append(differences, conformance(path+key+"/", nested, property, root, legacy)...)
	}

	return differences
}

func scalarConformance(path string, field reflect.StructField, property map[string]any, schema map[string]any, root map[string]any) []string {
	var differences []string
	var schemaSecret bool = property["secret"] == true

	if secret := catalog.SecretField(field); secret != schemaSecret {
		differences = append(differences, fmt.Sprintf("%s: secret is %v, the schema says %v", path, secret, schemaSecret))
	}

	tagValues, tagged := catalog.EnumValues(field)
	schemaValues, enumerated := schemaEnum(catalog.FieldKey(field), property, schema, root)

	slices.Sort(tagValues)
	slices.Sort(schemaValues)

	if tagged != enumerated || !slices.Equal(tagValues, schemaValues) {
		differences = append(differences, fmt.Sprintf("%s: enum is %q, the schema allows %q", path, tagValues, schemaValues))
	}

	return differences
}

// schemaEnum returns the values a property allows: its enum, or the values its oneOf branches set, as for auth_type.
func schemaEnum(key string, property map[string]any, schema map[string]any, root map[string]any) ([]string, bool) {
	var values []string

	if values, found := propertyEnum(property, root); found {
		return values, true
	}

	if reference, found := schema["$ref"].(string); found {
		schema = resolve(reference, root)
	}

	branches, _ := schema["oneOf"].([]any)

	for _, branch := range branches {
		object, _ := branch.(map[string]any)

		discriminator, _ := schemaProperties(object, root)[key]

		value, found := discriminator["const"]

		if !found {
			value, found = discriminator["default"]
		}

		if !found {
			return nil, false
		}

		values = append(values, fmt.Sprint(value))
	}

	return values, len(values) > 0
}

// propertyEnum returns the enum of a property, following $ref and allOf.
func propertyEnum(property map[string]any, root map[string]any) ([]string, bool) {
	var values []string

	if reference, found := property["$ref"].(string); found {
		return propertyEnum(resolve(reference, root), root)
	}

	if enum, found := property["enum"].([]any); found {
		for _, value := range enum {
			values = append(values, fmt.Sprint(value))
		}

		return values, true
	}

	branches, _ := property["allOf"].([]any)

	for _, branch := range branches {
		nested, _ := branch.(map[string]any)

		if values, found := propertyEnum(nested, root); found {
			return values, true
		}
	}

	return nil, false
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

// schemaRequired returns the required properties, excluding those of oneOf, anyOf and allOf branches, which a mapped
// struct merges and so can't require.
func schemaRequired(schema map[string]any, root map[string]any) []string {
	var required []string

	if reference, found := schema["$ref"].(string); found {
		schema = resolve(reference, root)
	}

	keys, _ := schema["required"].([]any)

	for _, key := range keys {
		if name, isName := key.(string); isName {
			required = append(required, name)
		}
	}

	return required
}

// dropsValidZero reports whether a field omits a valid zero value, such as false or 0, when written.
func dropsValidZero(field reflect.StructField) bool {
	var omitEmpty bool = catalog.FieldOmitEmpty(field)

	switch field.Type.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int64, reflect.Float64:
		return omitEmpty
	default:
		return false
	}
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

func mappedFields(mapped reflect.Type) map[string]reflect.StructField {
	var fields map[string]reflect.StructField = map[string]reflect.StructField{}

	mapped = structType(mapped)

	if mapped == nil {
		return fields
	}

	for index := range mapped.NumField() {
		var field reflect.StructField = mapped.Field(index)

		var name string = catalog.FieldKey(field)

		if catalog.FieldInline(field) {
			maps.Copy(fields, mappedFields(field.Type))
		} else if name != "" && name != "-" {
			fields[name] = field
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
