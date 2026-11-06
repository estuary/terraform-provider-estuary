package spectest

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

// AssertEnumTags checks each enum tag in S matches the list of its field type, such as sslmode.PostgresModes, typed
// fields carry a tag, and every list is used.
func AssertEnumTags[S any](test *testing.T, lists ...any) {
	test.Helper()

	var values map[reflect.Type][]string = map[reflect.Type][]string{}
	var used map[reflect.Type]bool = map[reflect.Type]bool{}

	for _, list := range lists {
		var value reflect.Value = reflect.ValueOf(list)

		if value.Kind() != reflect.Slice || value.Type().Elem().Kind() != reflect.String || value.Type().Elem().PkgPath() == "" {
			test.Errorf("list %T is not a slice of a named string type", list)

			continue
		}

		var listed []string = make([]string, value.Len())

		for index := range value.Len() {
			listed[index] = value.Index(index).String()
		}

		slices.Sort(listed)

		values[value.Type().Elem()] = listed
	}

	var differences []string = enumTags("", reflect.TypeFor[S](), values, used, map[reflect.Type]bool{})

	for _, element := range sortedTypes(values) {
		if !used[element] {
			differences = append(differences, fmt.Sprintf("[]%s: no enum tagged field of the spec uses it", element))
		}
	}

	if len(differences) > 0 {
		test.Errorf("enum tags differ from their lists:\n%s", joinLines(differences))
	}
}

func enumTags(path string, mapped reflect.Type, values map[reflect.Type][]string, used map[reflect.Type]bool, visited map[reflect.Type]bool) []string {
	var differences []string

	for mapped.Kind() == reflect.Pointer || mapped.Kind() == reflect.Slice || mapped.Kind() == reflect.Array || mapped.Kind() == reflect.Map {
		mapped = mapped.Elem()
	}

	if mapped.Kind() != reflect.Struct || visited[mapped] {
		return nil
	}

	visited[mapped] = true

	for index := range mapped.NumField() {
		var field reflect.StructField = mapped.Field(index)
		var key string = catalog.FieldKey(field)

		if key == "" {
			key = field.Name
		}

		var element reflect.Type = field.Type

		for element.Kind() == reflect.Pointer {
			element = element.Elem()
		}

		listed, listedType := values[element]
		tagValues, tagged := catalog.EnumValues(field)

		switch {
		case listedType && tagged:
			used[element] = true

			slices.Sort(tagValues)

			if !slices.Equal(tagValues, listed) {
				differences = append(differences, fmt.Sprintf("%s: enum tag is %q, []%s lists %q", path+key, tagValues, element, listed))
			}
		case listedType:
			differences = append(differences, fmt.Sprintf("%s: %s has a list but the field has no enum tag", path+key, element))
		}

		differences = append(differences, enumTags(path+key+"/", field.Type, values, used, visited)...)
	}

	return differences
}

func sortedTypes(values map[reflect.Type][]string) []reflect.Type {
	var types []reflect.Type

	for element := range values {
		types = append(types, element)
	}

	slices.SortFunc(types, func(left reflect.Type, right reflect.Type) int {
		return strings.Compare(left.String(), right.String())
	})

	return types
}
