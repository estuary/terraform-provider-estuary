package translator

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode"
)

type field struct {
	key         string
	attribute   string
	description string
	optional    bool
	kind        reflect.Type
}

func attributeName(key string) string {
	var runes []rune = []rune(key)
	var name strings.Builder

	for index, current := range runes {

		if index > 0 && unicode.IsUpper(current) {
			var previous rune = runes[index-1]
			var nextIsLower bool = index+1 < len(runes) && unicode.IsLower(runes[index+1])

			if unicode.IsLower(previous) || unicode.IsDigit(previous) || (unicode.IsUpper(previous) && nextIsLower) {
				name.WriteRune('_')
			}

		}

		name.WriteRune(unicode.ToLower(current))
	}

	return name.String()
}

func fields(structType reflect.Type) ([]field, error) {
	var mappedFields []field
	var attributes []string

	for index := range structType.NumField() {
		var structField reflect.StructField = structType.Field(index)
		var options []string = strings.Split(structField.Tag.Get("yaml"), ",")

		if options[0] == "" || options[0] == "-" {
			continue
		}

		var attribute string = attributeName(options[0])

		if slices.Contains(attributes, attribute) {
			return nil, fmt.Errorf("%s: attribute %s is mapped twice.", structType, attribute)
		}

		attributes = append(attributes, attribute)

		mappedFields = append(mappedFields, field{
			key:         options[0],
			attribute:   attribute,
			description: structField.Tag.Get("description"),
			optional:    slices.Contains(options[1:], "omitempty") || structField.Type.Kind() == reflect.Pointer,
			kind:        structField.Type,
		})
	}

	return mappedFields, nil
}
