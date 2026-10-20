package builder

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var testPasswordPath *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName("config").WithAttributeName("password")
var testPasswordHashPath *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName("config").WithAttributeName("password_hash")

func TestSecretsHash(test *testing.T) {
	hash, err := hashSecret("secret")

	if err != nil {
		test.Fatal(err)
	}

	other, err := hashSecret("secret")

	if err != nil {
		test.Fatal(err)
	}

	if strings.Contains(hash, "secret") {
		test.Errorf("hash = %q, want no plaintext secret", hash)
	}

	if hash == other {
		test.Error("hashes of the same secret are equal, want a new salt for each hash")
	}

	expectations := map[string]struct {
		secret string
		stored string
		want   bool
	}{
		"same secret":           {"secret", hash, true},
		"same secret, salted":   {"secret", other, true},
		"changed secret":        {"rotated", hash, false},
		"empty secret":          {"", hash, false},
		"other algorithm":       {"secret", strings.Replace(hash, secretsHashAlgorithm, "md5", 1), false},
		"malformed iterations":  {"secret", strings.Replace(hash, "$600000$", "$many$", 1), false},
		"malformed stored hash": {"secret", "secret", false},
		"empty stored hash":     {"secret", "", false},
	}

	for name, expected := range expectations {
		if got := verifySecret(expected.secret, expected.stored); got != expected.want {
			test.Errorf("%s: verifySecret = %v, want %v", name, got, expected.want)
		}
	}
}

func TestCodecSecrets(test *testing.T) {
	var spec CaptureSpec[testConfig, testResource] = testCaptureSpec

	codec, err := NewCaptureCodec[testConfig, testResource](testNamedConnector)

	if err != nil {
		test.Fatal(err)
	}

	spec.Endpoint.Connector.Config.Password = "secret"

	config, err := codec.Value("movies", spec)

	if err != nil {
		test.Fatal(err)
	}

	plan, err := codec.NullSecrets(config)

	if err != nil {
		test.Fatal(err)
	}

	if password, _ := attributeValue(plan, testPasswordPath); !password.IsNull() {
		test.Errorf("plan password = %v, want null", password)
	}

	merged, err := codec.WithConfigSecrets(plan, config)

	if err != nil {
		test.Fatal(err)
	}

	if !merged.Equal(config) {
		test.Errorf("plan with config secrets = %v, want the config", merged)
	}
}

func TestCodecSecretPath(test *testing.T) {
	codec, err := NewCaptureCodec[testConfig, testResource](testNamedConnector)

	if err != nil {
		test.Fatal(err)
	}

	var config *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName("config")

	expectations := map[string]struct {
		path   *tftypes.AttributePath
		isHash bool
	}{
		"hash of a write-only attribute":   {config.WithAttributeName("password_hash"), true},
		"write-only attribute":             {testPasswordPath, false},
		"hash suffix of a plain attribute": {config.WithAttributeName("host_hash"), false},
		"plain attribute":                  {config.WithAttributeName("host"), false},
	}

	for name, expected := range expectations {
		secret, isHash := codec.secretPath(expected.path)

		if isHash != expected.isHash || (isHash && !secret.Equal(testPasswordPath)) {
			test.Errorf("%s: secretPath = %v, %v, want %v", name, secret, isHash, expected.isHash)
		}
	}
}

func TestAttributePath(test *testing.T) {
	var attribute *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(1).WithAttributeName("labels").WithElementKeyString("main")

	if got, want := attributePath(attribute), path.Root("bindings").AtListIndex(1).AtName("labels").AtMapKey("main"); !got.Equal(want) {
		test.Errorf("attributePath = %s, want %s", got, want)
	}
}
