package flowctl

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-log/tflogtest"
	"gopkg.in/yaml.v3"
)

const privateKey string = "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEAsecretbody\nmoresecretline\n-----END RSA PRIVATE KEY-----\n"

// logSecrets are secrets YAML doesn't write raw: multiline, quoted or escaped.
var logSecrets = []string{privateKey, "it's: secret", "tab\there"}

func logSpec() catalog.CaptureSpec {
	return catalog.RawCapture(map[string]any{
		"endpoint": map[string]any{
			"connector": map[string]any{
				"image": "ghcr.io/estuary/source-mongodb:v1",
				"config": map[string]any{
					"address":   "mongodb://host:27017",
					"password":  logSecrets[1],
					"user_note": logSecrets[2],
					"networkTunnel": map[string]any{
						"sshForwarding": map[string]any{"privateKey": logSecrets[0]},
					},
				},
			},
		},
		"bindings": []any{},
	})
}

// assertNoSecretLogged fails when a log entry holds a secret or a line of one, in any encoding.
func assertNoSecretLogged(test *testing.T, output *bytes.Buffer, want string) {
	entries, err := tflogtest.MultilineJSONDecode(output)

	if err != nil {
		test.Fatal(err)
	}

	var logged strings.Builder

	for _, entry := range entries {
		content, err := yaml.Marshal(entry)

		if err != nil {
			test.Fatal(err)
		}

		logged.Write(content)

		for _, value := range entry {
			if text, isText := value.(string); isText {
				logged.WriteString(text + "\n")
			}
		}
	}

	for _, secret := range logSecrets {
		encoded, _ := json.Marshal(secret)

		for _, part := range append([]string{secret, strings.ReplaceAll(secret, "'", "''"), string(encoded[1 : len(encoded)-1])}, strings.Split(secret, "\n")...) {
			if part = strings.TrimSpace(part); len(part) >= 4 && strings.Contains(logged.String(), part) {
				test.Errorf("logs hold %q:\n%s", part, logged.String())
			}
		}
	}

	if !strings.Contains(logged.String(), want) {
		test.Errorf("logs lost %q:\n%s", want, logged.String())
	}
}

func fakeFlowctl(test *testing.T, script string) *Client {
	var binary string = filepath.Join(test.TempDir(), "flowctl")

	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		test.Fatal(err)
	}

	return NewClient(binary, "<api-token>", "")
}

func TestPublishLogsNoSecret(test *testing.T) {
	var output bytes.Buffer
	var client *Client = fakeFlowctl(test, "#!/bin/sh\nexit 0\n")
	var ctx context.Context = catalog.WithSecrets(tflogtest.RootLogger(context.Background(), &output), logSecrets...)

	if err := client.PublishCapture(ctx, "acmeCo/capture", logSpec(), nil); err != nil {
		test.Fatal(err)
	}

	assertNoSecretLogged(test, &output, "mongodb://host:27017")
}

func TestDiscoverLogsNoSecret(test *testing.T) {
	var output bytes.Buffer
	var client *Client = fakeFlowctl(test, "#!/bin/sh\neval spec=\\${$#}\necho 'captures: {}' > \"$spec\"\n")
	var ctx context.Context = catalog.WithSecrets(tflogtest.RootLogger(context.Background(), &output), logSecrets...)

	if _, err := client.DiscoverCapture(ctx, "acmeCo/capture", logSpec()); err != nil {
		test.Fatal(err)
	}

	assertNoSecretLogged(test, &output, "mongodb://host:27017")
}

func TestDiscoverLogsMaskSensitiveKeys(test *testing.T) {
	var output bytes.Buffer
	var client *Client = fakeFlowctl(test, "#!/bin/sh\neval spec=\\${$#}\necho 'captures: {}' > \"$spec\"\n")
	var ctx context.Context = catalog.WithSecrets(tflogtest.RootLogger(context.Background(), &output), logSecrets[2])

	if _, err := client.DiscoverCapture(ctx, "acmeCo/capture", logSpec()); err != nil {
		test.Fatal(err)
	}

	assertNoSecretLogged(test, &output, "mongodb://host:27017")
}

func TestExecCommandMasksEncodedSecretsInErrors(test *testing.T) {
	var client *Client = fakeFlowctl(test, "#!/bin/sh\nprintf 'config:\\n  key: |\\n    MIIEpAIBAAKCAQEAsecretbody\\n  password: '\"'\"'it'\"''\"'s: secret'\"'\"'\\n' >&2\nexit 1\n")

	_, err := client.execCommand(catalog.WithSecrets(context.Background(), logSecrets...), "catalog", "publish")

	if err == nil {
		test.Fatal("execCommand succeeded, want the script failure")
	}

	for _, leaked := range []string{"MIIEpAIBAAKCAQEAsecretbody", "it''s: secret"} {
		if strings.Contains(err.Error(), leaked) {
			test.Errorf("error still holds %q: %v", leaked, err)
		}
	}
}

func TestMaskedSpecHidesUnparsableSpecs(test *testing.T) {
	if masked := maskedSpec([]byte("password: [unclosed"), nil); masked != "<unparsable spec>" {
		test.Errorf("maskedSpec = %q, want the spec hidden", masked)
	}
}
