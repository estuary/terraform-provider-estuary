package translator

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

type field struct {
	key                string
	attribute          string
	description        string
	optional           bool
	secret             bool
	forbidsCredentials bool
	enum               []string
	kind               reflect.Type
}

// AttributeName returns the Terraform attribute name of a YAML key, in snake case.
func AttributeName(key string) string {
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
		var key string = catalog.FieldKey(structField)

		if catalog.FieldInline(structField) {
			inlined, err := fields(structField.Type)

			if err != nil {
				return nil, err
			}

			mappedFields = append(mappedFields, inlined...)

			continue
		}

		if key == "" || key == "-" {
			continue
		}

		if err := catalog.ValidateTags(structField); err != nil {
			return nil, fmt.Errorf("%s: %w", structType, err)
		}

		var attribute string = AttributeName(key)

		if slices.Contains(attributes, attribute) {
			return nil, fmt.Errorf("%s: attribute %s is mapped twice", structType, attribute)
		}

		attributes = append(attributes, attribute)

		enum, _ := catalog.EnumValues(structField)

		mappedFields = append(mappedFields, field{
			key:                key,
			attribute:          attribute,
			description:        catalog.FieldDescription(structField),
			optional:           catalog.FieldOmitEmpty(structField) || structField.Type.Kind() == reflect.Pointer,
			secret:             catalog.SecretField(structField),
			forbidsCredentials: catalog.ForbidsCredentials(structField),
			enum:               enum,
			kind:               structField.Type,
		})
	}

	return mappedFields, nil
}
