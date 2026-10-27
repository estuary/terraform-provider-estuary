package catalog

import (
	"fmt"
	"reflect"
	"strings"
)

// sensitiveKeys are key fragments naming a secret.
var sensitiveKeys = []string{
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

// ValidateTags rejects secret and enum tags on non-string fields.
func ValidateTags(field reflect.StructField) error {
	_, secret := field.Tag.Lookup("secret")
	_, enum := field.Tag.Lookup("enum")

	if (secret || enum) && !stringField(field) {
		return fmt.Errorf("field %s: secret and enum tags only apply to strings, not %s", field.Name, field.Type)
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
