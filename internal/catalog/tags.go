package catalog

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// sensitiveKeys are key fragments naming a secret.
var sensitiveKeys []string = []string{
	"password",
	"secret",
	"token",
	"private",
	"credential",
	"api_key",
	"client_key",
}

// IsSensitiveKey reports whether the key names a secret.
func IsSensitiveKey(key string) bool {
	for _, sensitiveKey := range sensitiveKeys {
		if strings.Contains(strings.ToLower(key), sensitiveKey) {
			return true
		}
	}

	return false
}

// FieldKey returns a spec field's key from its yaml tag.
func FieldKey(field reflect.StructField) string {
	return strings.Split(field.Tag.Get("yaml"), ",")[0]
}

// FieldInline reports whether a spec field's yaml tag inlines its struct fields into the parent.
func FieldInline(field reflect.StructField) bool {
	return yamlOption(field, "inline")
}

// FieldOmitEmpty reports whether a spec field's yaml tag omits its empty value.
func FieldOmitEmpty(field reflect.StructField) bool {
	return yamlOption(field, "omitempty")
}

// FieldDescription returns a spec field's description tag, empty without one.
func FieldDescription(field reflect.StructField) string {
	return field.Tag.Get("description")
}

// BackfillModeField reports whether a string field is a binding resource's backfill mode, by its backfill:"mode" tag.
func BackfillModeField(field reflect.StructField) bool {
	return stringField(field) && field.Tag.Get("backfill") == "mode"
}

func yamlOption(field reflect.StructField, option string) bool {
	return slices.Contains(strings.Split(field.Tag.Get("yaml"), ",")[1:], option)
}

// SecretField reports whether a string field is secret, by its secret tag or its key.
func SecretField(field reflect.StructField) bool {
	return stringField(field) && (field.Tag.Get("secret") == "true" || IsSensitiveKey(FieldKey(field)))
}

// EnumValues returns the comma-separated values of a field's enum tag, reporting false without one.
func EnumValues(field reflect.StructField) ([]string, bool) {
	values, found := field.Tag.Lookup("enum")

	if !found {
		return nil, false
	}

	return strings.Split(values, ","), true
}

// ForbidsCredentials reports whether a string field, such as a connection URI, must not embed credentials, by its
// credentials:"forbidden" tag.
func ForbidsCredentials(field reflect.StructField) bool {
	return stringField(field) && field.Tag.Get("credentials") == "forbidden"
}

// ValidateTags rejects secret, enum, credentials and backfill tags on non-string fields.
func ValidateTags(field reflect.StructField) error {
	for _, tag := range []string{"secret", "enum", "credentials", "backfill"} {
		if _, found := field.Tag.Lookup(tag); found && !stringField(field) {
			return fmt.Errorf("field %s: secret, enum, credentials and backfill tags only apply to strings, not %s", field.Name, field.Type)
		}
	}

	return nil
}

func stringField(field reflect.StructField) bool {
	var kind reflect.Type = field.Type

	for kind.Kind() == reflect.Pointer {
		kind = kind.Elem()
	}

	return kind.Kind() == reflect.String
}
