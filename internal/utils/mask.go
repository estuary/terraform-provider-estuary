package utils

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
)

// mask replaces every secret masked in logs and errors.
const mask string = "***"

// minimumLineLength is the shortest multiline secret line masked on its own, so masking doesn't mangle other text.
const minimumLineLength int = 4

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

// SecretForms returns each secret's forms in text, longest first: raw, YAML/JSON quoted and escaped, and each line of a
// multiline secret, which YAML writes as an indented block.
func SecretForms(secrets []string) []string {
	var forms []string

	for _, secret := range secrets {
		if secret == "" {
			continue
		}

		forms = append(forms, secret, strings.ReplaceAll(secret, "'", "''"))

		if encoded, err := json.Marshal(secret); err == nil {
			forms = append(forms, string(encoded[1:len(encoded)-1]))
		}

		for _, line := range strings.Split(secret, "\n") {
			if line = strings.TrimSpace(line); len(line) >= minimumLineLength {
				forms = append(forms, line)
			}
		}
	}

	slices.SortFunc(forms, func(left string, right string) int {
		return cmp.Compare(len(right), len(left))
	})

	return slices.Compact(forms)
}

// MaskStrings replaces every form of every secret in text, see SecretForms.
func MaskStrings(text string, secrets []string) string {
	for _, form := range SecretForms(secrets) {
		text = strings.ReplaceAll(text, form, mask)
	}

	return text
}

// MaskDocument returns a copy of a YAML document with sensitive key values and embedded secrets masked, for logs.
func MaskDocument(value any, secrets []string) any {
	switch typed := value.(type) {
	case map[string]any:
		var masked map[string]any = make(map[string]any, len(typed))

		for key, nested := range typed {
			if _, isString := nested.(string); isString && IsSensitiveKey(key) {
				masked[key] = mask
			} else {
				masked[key] = MaskDocument(nested, secrets)
			}
		}

		return masked
	case []any:
		var masked []any = make([]any, len(typed))

		for index, item := range typed {
			masked[index] = MaskDocument(item, secrets)
		}

		return masked
	case string:
		return MaskStrings(typed, secrets)
	}

	return value
}
