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

// DeltaUpdatesField reports whether a bool field is a binding resource's delta updates, by its connector x-delta-updates:"true" tag.
func DeltaUpdatesField(field reflect.StructField) bool {
	return field.Type.Kind() == reflect.Bool && field.Tag.Get("x-delta-updates") == "true"
}

// SchemaNameField reports whether a string field is a binding resource's schema, by its connector x-schema-name:"true" tag.
func SchemaNameField(field reflect.StructField) bool {
	return stringField(field) && field.Tag.Get("x-schema-name") == "true"
}

// CollectionNameField reports whether a string field is a binding resource's table, by its connector x-collection-name:"true" tag.
func CollectionNameField(field reflect.StructField) bool {
	return stringField(field) && field.Tag.Get("x-collection-name") == "true"
}

// DefaultNamespaceField reports whether a string field is the endpoint's default schema of bindings, by its
// x-default-namespace:"true" tag.
func DefaultNamespaceField(field reflect.StructField) bool {
	return stringField(field) && field.Tag.Get("x-default-namespace") == "true"
}

func yamlOption(field reflect.StructField, option string) bool {
	return slices.Contains(strings.Split(field.Tag.Get("yaml"), ",")[1:], option)
}

// SecretField reports whether a string field is secret, by its secret tag, else by its key. secret:"false" keeps a
// field whose key looks sensitive, such as tokenUrl, readable.
func SecretField(field reflect.StructField) bool {
	if secret, tagged := field.Tag.Lookup("secret"); tagged && stringField(field) {
		return secret == "true"
	}

	return stringField(field) && IsSensitiveKey(FieldKey(field))
}

// EnumValues returns the comma-separated values of a field's enum tag, reporting false without one.
func EnumValues(field reflect.StructField) ([]string, bool) {
	values, found := field.Tag.Lookup("enum")

	if !found {
		return nil, false
	}

	return strings.Split(values, ","), true
}

// Credentials is which credentials a string field, such as a connection URI, must not embed, by its credentials tag.
type Credentials string

const (
	CredentialsAllowed   Credentials = ""
	CredentialsForbidden Credentials = "forbidden"
	CredentialsPassword  Credentials = "password"
)

// FieldCredentials returns which credentials a string field must not embed, CredentialsAllowed without credentials tag.
func FieldCredentials(field reflect.StructField) Credentials {
	if !stringField(field) {
		return CredentialsAllowed
	}

	return Credentials(field.Tag.Get("credentials"))
}

// ValidateTags rejects secret, enum, credentials, backfill, x-schema-name, x-collection-name and x-default-namespace tags
// on non-string fields, x-delta-updates tags on non-bool fields, and unknown credentials tags.
func ValidateTags(field reflect.StructField) error {
	for _, tag := range []string{"secret", "enum", "credentials", "backfill", "x-schema-name", "x-collection-name", "x-default-namespace"} {
		if _, found := field.Tag.Lookup(tag); found && !stringField(field) {
			return fmt.Errorf("field %s: secret, enum, credentials, backfill, x-schema-name, x-collection-name and x-default-namespace tags only apply to strings, not %s", field.Name, field.Type)
		}
	}

	if _, found := field.Tag.Lookup("x-delta-updates"); found && field.Type.Kind() != reflect.Bool {
		return fmt.Errorf("field %s: x-delta-updates tag only applies to bools, not %s", field.Name, field.Type)
	}

	if credentials, found := field.Tag.Lookup("credentials"); found && Credentials(credentials) != CredentialsForbidden && Credentials(credentials) != CredentialsPassword {
		return fmt.Errorf("field %s: credentials tag must be %q or %q, not %q", field.Name, CredentialsForbidden, CredentialsPassword, credentials)
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
