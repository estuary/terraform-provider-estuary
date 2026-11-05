package flowctl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"gopkg.in/yaml.v3"
)

// secretSamples are secrets YAML and JSON don't write raw: multiline, quoted or escaped.
var secretSamples = []string{privateKey, "it's: secret", "tab\there", `pa"ss\word`, "<api-token>"}

func TestMaskStrings(test *testing.T) {
	var masked string = maskStrings("auth failed for hunter2 at host", secretPatterns([]string{"hunter2", ""}))

	if masked != "auth failed for *** at host" {
		test.Errorf("masked = %q", masked)
	}
}

func TestMaskStringsMasksEncodedSecrets(test *testing.T) {
	for _, secret := range secretSamples {
		content, err := yaml.Marshal(map[string]any{"config": map[string]any{"value": secret}})

		if err != nil {
			test.Fatal(err)
		}

		encoded, err := json.Marshal(map[string]any{"value": secret})

		if err != nil {
			test.Fatal(err)
		}

		for _, text := range []string{string(content), string(encoded)} {
			var masked string = maskStrings(text, secretPatterns([]string{secret}))

			if leaked := leakedPart(masked, secret); leaked != "" {
				test.Errorf("masked %q still holds %q", masked, leaked)
			}
		}
	}
}

// urlSecrets are secrets URLs and JSON escape: reserved and HTML characters, spaces, non-ASCII and non-BMP characters.
var urlSecrets = []string{`p@ss"w0rd/secret`, `p<"x&y>`, "two words+plus", "pässwörd", "🔑key:with;semi=colon"}

// secretEncodings returns secret as each encoder flowctl and connectors may write it, by encoding name.
func secretEncodings(secret string) map[string]string {
	var plainJSON bytes.Buffer
	var encoder *json.Encoder = json.NewEncoder(&plainJSON)
	var lowerPercent strings.Builder
	var asciiJSON strings.Builder

	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(secret)

	htmlJSON, _ := json.Marshal(secret)

	for _, octet := range []byte(secret) {
		fmt.Fprintf(&lowerPercent, "%%%02x", octet)
	}

	for _, unit := range utf16.Encode([]rune(secret)) {
		fmt.Fprintf(&asciiJSON, `\u%04X`, unit)
	}

	var userInfo string = (&url.URL{Scheme: "postgres", User: url.UserPassword("flow", secret), Host: "123.4.567.89"}).String()
	var plainQuoted string = strings.TrimSuffix(plainJSON.String(), "\n")

	return map[string]string{
		"raw":               secret,
		"Go JSON":           string(htmlJSON[1 : len(htmlJSON)-1]),
		"JSON without HTML": plainQuoted[1 : len(plainQuoted)-1],
		"JSON \\u escapes":  asciiJSON.String(),
		"URL user info":     strings.TrimSuffix(strings.TrimPrefix(userInfo, "postgres://flow:"), "@123.4.567.89"),
		"URL query":         url.QueryEscape(secret),
		"URL path":          url.PathEscape(secret),
		"lowercase percent": lowerPercent.String(),
	}
}

func TestMaskStringsMasksURLAndJSONEncodedSecrets(test *testing.T) {
	for _, secret := range urlSecrets {
		for name, encoded := range secretEncodings(secret) {
			var text string = "auth failed for [" + encoded + "] at host"

			if masked := maskStrings(text, secretPatterns([]string{secret})); masked != "auth failed for [***] at host" {
				test.Errorf("%s of %q: masked = %q, want only the secret masked", name, secret, masked)
			}
		}
	}
}

func TestMaskStringsMasksLongerSecretsFirst(test *testing.T) {
	for _, secrets := range [][]string{{"abc", "abcdef"}, {"abcdef", "abc"}} {
		if masked := maskStrings("token abcdef leaked, abc too", secretPatterns(secrets)); masked != "token *** leaked, *** too" {
			test.Errorf("secrets %q: masked = %q, want no part of the longer one left", secrets, masked)
		}
	}
}

func TestMaskStringsKeepsOtherText(test *testing.T) {
	var text string = "ABC abd %61%62 a+b a\\u0062 hunter"

	if masked := maskStrings(text, secretPatterns([]string{"abc", "hunter2"})); masked != text {
		test.Errorf("masked = %q, want text without the secrets kept", masked)
	}
}

func TestMaskStringsKeepsShortLines(test *testing.T) {
	var masked string = maskStrings("a key, b value", secretPatterns([]string{"a\nb\nlonger line"}))

	if masked != "a key, b value" {
		test.Errorf("masked = %q, want the short lines of the secret kept", masked)
	}
}

func TestSecretPatternsCompileEachPartOnce(test *testing.T) {
	var patterns []*regexp.Regexp = secretPatterns([]string{"abcd", "wxyz", "abcd", "id\nabcd\n"})

	if len(patterns) != 3 {
		test.Errorf("patterns = %v, want one per distinct secret or line: the multiline secret, abcd and wxyz", patterns)
	}
}

func TestMaskDocument(test *testing.T) {
	var document map[string]any = map[string]any{
		"endpoint": map[string]any{
			"config": map[string]any{
				"address":  "mongodb://host:27017",
				"password": "unlisted",
				"port":     27017,
				"networkTunnel": map[string]any{
					"sshForwarding": map[string]any{"privateKey": privateKey},
				},
				"note":    "uses hunter2 inside",
				"secrets": []any{"hunter2"},
			},
		},
	}
	var want map[string]any = map[string]any{
		"endpoint": map[string]any{
			"config": map[string]any{
				"address":  "mongodb://host:27017",
				"password": "***",
				"port":     27017,
				"networkTunnel": map[string]any{
					"sshForwarding": map[string]any{"privateKey": "***"},
				},
				"note":    "uses *** inside",
				"secrets": []any{"***"},
			},
		},
	}

	if masked := maskDocument(document, secretPatterns([]string{"hunter2"})); !reflect.DeepEqual(masked, want) {
		test.Errorf("masked = %v, want %v", masked, want)
	}

	if utils.Lookup(document, "endpoint", "config")["password"] != "unlisted" {
		test.Error("maskDocument changed the document it masks")
	}
}

// leakedPart returns the part of secret, or of its individually masked lines, still in text, empty when none.
func leakedPart(text string, secret string) string {
	for _, part := range append([]string{secret}, strings.Split(secret, "\n")...) {
		if len(strings.TrimSpace(part)) >= 4 && strings.Contains(text, strings.TrimSpace(part)) {
			return part
		}
	}

	return ""
}
