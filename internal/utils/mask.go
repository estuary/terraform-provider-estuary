package utils

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// sensitiveKeys are the key fragments whose string values are masked in logged specs.
var sensitiveKeys = []string{
	"password",
	"secret",
	"token",
	"private",
	"credential",
	"api_key",
	"client_key",
}

func maskValue(key string, value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for nestedKey, nestedValue := range typed {
			typed[nestedKey] = maskValue(nestedKey, nestedValue)
		}
	case []any:
		for index, item := range typed {
			typed[index] = maskValue(key, item)
		}
	case string:
		if IsSensitiveKey(key) {
			return "***"
		}
	}

	return value
}

func collectSecrets(key string, value any, secrets []string) []string {
	switch typed := value.(type) {
	case map[string]any:
		for nestedKey, nestedValue := range typed {
			secrets = collectSecrets(nestedKey, nestedValue, secrets)
		}
	case []any:
		for _, item := range typed {
			secrets = collectSecrets(key, item, secrets)
		}
	case string:
		if typed != "" && IsSensitiveKey(key) {
			secrets = append(secrets, typed)
		}
	}

	return secrets
}

// IsSensitiveKey reports whether the key names a secret, such as a password, token or private key.
func IsSensitiveKey(key string) bool {
	for _, sensitiveKey := range sensitiveKeys {
		if strings.Contains(strings.ToLower(key), sensitiveKey) {
			return true
		}
	}

	return false
}

// MaskYAML returns the YAML content with sensitive string values replaced, for logging.
func MaskYAML(content []byte) []byte {
	var document any

	if err := yaml.Unmarshal(content, &document); err != nil {
		return []byte("<unparseable spec>")
	}

	masked, err := yaml.Marshal(maskValue("", document))

	if err != nil {
		return []byte("<unmarshalable spec>")
	}

	return masked
}

// SecretValues returns the non empty string values held under sensitive keys of the YAML content.
func SecretValues(content []byte) []string {
	var document any

	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil
	}

	return collectSecrets("", document, nil)
}

// MaskStrings returns the text with every non empty secret replaced.
func MaskStrings(text string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "***")
		}
	}

	return text
}
