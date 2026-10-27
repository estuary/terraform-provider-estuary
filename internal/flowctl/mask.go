package flowctl

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"gopkg.in/yaml.v3"
)

// mask replaces every secret masked in logs and errors.
const mask string = "***"

// minimumLineLength is the shortest multiline secret line masked on its own, so masking doesn't mangle other text.
const minimumLineLength int = 4

// secretForms returns each secret's forms in text, longest first: raw, YAML/JSON quoted and escaped, and each line of a
// multiline secret, which YAML writes as an indented block.
func secretForms(secrets []string) []string {
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

// maskStrings replaces every form of every secret in text, see secretForms.
func maskStrings(text string, secrets []string) string {
	for _, form := range secretForms(secrets) {
		text = strings.ReplaceAll(text, form, mask)
	}

	return text
}

// maskDocument returns a copy of a YAML document with sensitive key values and embedded secrets masked, for logs.
func maskDocument(value any, secrets []string) any {
	switch typed := value.(type) {
	case map[string]any:
		var masked map[string]any = make(map[string]any, len(typed))

		for key, nested := range typed {
			if _, isString := nested.(string); isString && catalog.IsSensitiveKey(key) {
				masked[key] = mask
			} else {
				masked[key] = maskDocument(nested, secrets)
			}
		}

		return masked
	case []any:
		var masked []any = make([]any, len(typed))

		for index, item := range typed {
			masked[index] = maskDocument(item, secrets)
		}

		return masked
	case string:
		return maskStrings(typed, secrets)
	}

	return value
}

// maskedSpec returns the spec file content masked for logs, see maskDocument, empty when it doesn't parse.
func maskedSpec(content []byte, secrets []string) string {
	var document any

	if err := yaml.Unmarshal(content, &document); err != nil {
		return "<unparsable spec>"
	}

	masked, err := yaml.Marshal(maskDocument(document, secrets))

	if err != nil {
		return "<unparsable spec>"
	}

	return string(masked)
}
