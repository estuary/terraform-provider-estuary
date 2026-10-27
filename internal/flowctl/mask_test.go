package flowctl

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"gopkg.in/yaml.v3"
)

// secretSamples are secrets YAML and JSON don't write raw: multiline, quoted or escaped.
var secretSamples = []string{privateKey, "it's: secret", "tab\there", `pa"ss\word`, "<api-token>"}

func TestMaskStrings(test *testing.T) {
	var masked string = maskStrings("auth failed for hunter2 at host", []string{"hunter2", ""})

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
			var masked string = maskStrings(text, []string{secret})

			if leaked := leakedPart(masked, secret); leaked != "" {
				test.Errorf("masked %q still holds %q", masked, leaked)
			}
		}
	}
}

func TestMaskStringsKeepsShortLines(test *testing.T) {
	var masked string = maskStrings("a key, b value", []string{"a\nb\nlonger line"})

	if masked != "a key, b value" {
		test.Errorf("masked = %q, want the short lines of the secret kept", masked)
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

	if masked := maskDocument(document, []string{"hunter2"}); !reflect.DeepEqual(masked, want) {
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
