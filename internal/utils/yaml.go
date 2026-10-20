package utils

import "gopkg.in/yaml.v3"

// Lookup returns the object nested under keys in a YAML document, nil when any key is missing or not an object.
func Lookup(value any, keys ...string) map[string]any {
	for _, key := range keys {
		object, _ := value.(map[string]any)

		value = object[key]
	}

	object, _ := value.(map[string]any)

	return object
}

// YAMLDocument returns value as a YAML document, through its YAML tags, getting the Go object and dumping a map with the YAML kind fields.
func YAMLDocument(value any) (map[string]any, error) {
	var document map[string]any

	content, err := yaml.Marshal(value)

	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, err
	}

	return document, nil
}
