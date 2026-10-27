package utils

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

// Lookup returns the object nested under keys, nil when any key is missing or not an object.
func Lookup(value any, keys ...string) map[string]any {
	for _, key := range keys {
		object, _ := value.(map[string]any)

		value = object[key]
	}

	object, _ := value.(map[string]any)

	return object
}

// YAMLDocument converts value to an untyped YAML document through its YAML tags.
func YAMLDocument(value any) (map[string]any, error) {
	return ConvertYAML[map[string]any](value, false)
}

// ConvertYAML converts value to T through their YAML tags, strict failing on keys T doesn't map.
func ConvertYAML[T any](value any, strict bool) (T, error) {
	var converted T

	content, err := yaml.Marshal(value)

	if err != nil {
		return converted, err
	}

	var decoder *yaml.Decoder = yaml.NewDecoder(bytes.NewReader(content))

	decoder.KnownFields(strict)

	err = decoder.Decode(&converted)

	return converted, err
}
