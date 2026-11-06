package flowctl

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"gopkg.in/yaml.v3"
)

// mask replaces every secret masked in logs and errors.
const mask string = "***"

// minimumLineLength is the shortest multiline secret line masked on its own, so masking doesn't mangle other text.
const minimumLineLength int = 4

// jsonEscapes are the short escapes JSON and YAML double-quoted strings write for a character.
var jsonEscapes map[rune]string = map[rune]string{'"': `\"`, '\\': `\\`, '/': `\/`, '\n': `\n`, '\t': `\t`, '\r': `\r`, '\b': `\b`, '\f': `\f`}

// secretPatterns returns a pattern per secret, longest first, matching it whatever the encoding each character is
// written in, see characterPattern, so a secret in a URL or JSON is masked even when only some characters are escaped.
// Each line of a multiline secret, which YAML writes as an indented block, gets its own pattern.
func secretPatterns(secrets []string) []*regexp.Regexp {
	var parts []string

	for _, secret := range secrets {
		if secret == "" {
			continue
		}

		parts = append(parts, secret)

		for _, line := range strings.Split(secret, "\n") {
			if line = strings.TrimSpace(line); len(line) >= minimumLineLength {
				parts = append(parts, line)
			}
		}
	}

	slices.SortFunc(parts, func(left string, right string) int {
		return cmp.Or(cmp.Compare(len(right), len(left)), strings.Compare(left, right))
	})

	var patterns []*regexp.Regexp

	for _, part := range slices.Compact(parts) {
		var pattern strings.Builder

		for _, character := range part {
			pattern.WriteString(characterPattern(character))
		}

		patterns = append(patterns, regexp.MustCompile(pattern.String()))
	}

	return patterns
}

// characterPattern matches a character raw, percent-encoded in either hex case, as a JSON \u escape or short escape,
// doubled as a YAML single-quoted quote, or as + for a space in a query.
func characterPattern(character rune) string {
	var alternatives []string = []string{regexp.QuoteMeta(string(character))}
	var percent strings.Builder
	var unicode strings.Builder

	for _, octet := range []byte(string(character)) {
		fmt.Fprintf(&percent, "%%%02x", octet)
	}

	for _, unit := range utf16.Encode([]rune{character}) {
		fmt.Fprintf(&unicode, `\\u%04x`, unit)
	}

	alternatives = append(alternatives, "(?i:"+percent.String()+")", "(?i:"+unicode.String()+")")

	if escape, found := jsonEscapes[character]; found {
		alternatives = append(alternatives, regexp.QuoteMeta(escape))
	}

	switch character {
	case '\'':
		alternatives = append(alternatives, "''")
	case ' ':
		alternatives = append(alternatives, `\+`)
	}

	return "(?:" + strings.Join(alternatives, "|") + ")"
}

// maskStrings replaces every match of the secret patterns in text, see secretPatterns.
func maskStrings(text string, patterns []*regexp.Regexp) string {
	for _, pattern := range patterns {
		text = pattern.ReplaceAllLiteralString(text, mask)
	}

	return text
}

// maskDocument returns a copy of a YAML document with sensitive key values and secret pattern matches masked, for logs.
func maskDocument(value any, patterns []*regexp.Regexp) any {
	switch typed := value.(type) {
	case map[string]any:
		var masked map[string]any = make(map[string]any, len(typed))

		for key, nested := range typed {
			if _, isString := nested.(string); isString && catalog.IsSensitiveKey(key) {
				masked[key] = mask
			} else {
				masked[key] = maskDocument(nested, patterns)
			}
		}

		return masked
	case []any:
		var masked []any = make([]any, len(typed))

		for index, item := range typed {
			masked[index] = maskDocument(item, patterns)
		}

		return masked
	case string:
		return maskStrings(typed, patterns)
	}

	return value
}

// maskedSpec returns the spec file content masked for logs, see maskDocument, empty when it doesn't parse.
func maskedSpec(content []byte, patterns []*regexp.Regexp) string {
	var document any

	if err := yaml.Unmarshal(content, &document); err != nil {
		return "<unparsable spec>"
	}

	masked, err := yaml.Marshal(maskDocument(document, patterns))

	if err != nil {
		return "<unparsable spec>"
	}

	return string(masked)
}
