package utils_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"gopkg.in/yaml.v3"
)

const privateKey string = "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEAsecretbody\nmoresecretline\n-----END RSA PRIVATE KEY-----\n"

// secretSamples are secrets YAML and JSON don't write raw: multiline, quoted or escaped.
var secretSamples = []string{privateKey, "it's: secret", "tab\there", `pa"ss\word`, "<api-token>"}

func TestMaskStrings(test *testing.T) {
	var masked string = utils.MaskStrings("auth failed for hunter2 at host", []string{"hunter2", ""})

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
			var masked string = utils.MaskStrings(text, []string{secret})

			if leaked := leakedPart(masked, secret); leaked != "" {
				test.Errorf("masked %q still holds %q", masked, leaked)
			}
		}
	}
}

func TestMaskStringsKeepsShortLines(test *testing.T) {
	var masked string = utils.MaskStrings("a key, b value", []string{"a\nb\nlonger line"})

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

	if masked := utils.MaskDocument(document, []string{"hunter2"}); !reflect.DeepEqual(masked, want) {
		test.Errorf("masked = %v, want %v", masked, want)
	}

	if utils.Lookup(document, "endpoint", "config")["password"] != "unlisted" {
		test.Error("MaskDocument changed the document it masks")
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

func TestIsSensitiveKey(test *testing.T) {
	expectations := map[string]bool{
		"password":        true,
		"privateKey":      true,
		"ssl_client_key":  true,
		"api_key":         true,
		"sslmode":         false,
		"aws_access_key":  false,
		"ssl_server_cert": false,
	}

	for key, want := range expectations {
		if got := utils.IsSensitiveKey(key); got != want {
			test.Errorf("%s: IsSensitiveKey = %v, want %v", key, got, want)
		}
	}
}
