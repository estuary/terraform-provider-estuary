package builder

import (
	"strings"
	"testing"
	"time"

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

	fewerIterations, err := secretHash("secret", make([]byte, secretsHashSaltSize), 1000)

	if err != nil {
		test.Fatal(err)
	}

	longerSalt, err := secretHash("secret", make([]byte, 2*secretsHashSaltSize), secretsHashIterations)

	if err != nil {
		test.Fatal(err)
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
		"fewer iterations":      {"secret", fewerIterations, true},
		"more iterations":       {"secret", strings.Replace(hash, "$600000$", "$2147483647$", 1), false},
		"another salt size":     {"secret", longerSalt, false},
	}

	for name, expected := range expectations {
		var start time.Time = time.Now()

		if got := verifySecret(expected.secret, expected.stored); got != expected.want {
			test.Errorf("%s: verifySecret = %v, want %v", name, got, expected.want)
		}

		if elapsed := time.Since(start); elapsed > 10*time.Second {
			test.Errorf("%s: verifySecret took %s, want stored hash parameters bounded", name, elapsed)
		}
	}
}

func TestVerifySecretDerivesOnce(test *testing.T) {
	hash, err := hashSecret("secret")

	if err != nil {
		test.Fatal(err)
	}

	if !verifySecret("secret", hash) || verifySecret("rotated", hash) {
		test.Fatal("verifySecret matched the wrong secret")
	}

	var start time.Time = time.Now()

	for range 100 {
		if !verifySecret("secret", hash) {
			test.Fatal("verifySecret stopped matching the secret once cached")
		}
	}

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		test.Errorf("100 verifications took %s, want the matching secret derived once", elapsed)
	}

	if verifySecret("rotated", hash) {
		test.Error("verifySecret matched another secret against a cached hash")
	}
}

func TestCodecSecrets(test *testing.T) {
	var spec CaptureSpec[testConfig, testResource] = testCaptureSpec

	codec, err := newCodec[captureDocument[testConfig, testResource]](testNamedConnector, captureType[testResource]())

	if err != nil {
		test.Fatal(err)
	}

	spec.Endpoint.Connector.Config.Password = "secret"

	config, err := codec.Value("movies", spec)

	if err != nil {
		test.Fatal(err)
	}

	plan, err := codec.secrets.nullSecrets(config)

	if err != nil {
		test.Fatal(err)
	}

	if password, _ := attributeValue(plan, testPasswordPath); !password.IsNull() {
		test.Errorf("plan password = %v, want null", password)
	}

	merged, err := codec.secrets.withConfigSecrets(plan, config)

	if err != nil {
		test.Fatal(err)
	}

	if !merged.Equal(config) {
		test.Errorf("plan with config secrets = %v, want the config", merged)
	}
}

func TestCodecSecretsRemoved(test *testing.T) {
	var spec CaptureSpec[testConfig, testResource] = testCaptureSpec

	codec, err := newCodec[captureDocument[testConfig, testResource]](testNamedConnector, captureType[testResource]())

	if err != nil {
		test.Fatal(err)
	}

	spec.Endpoint.Connector.Config.Password = "secret"

	config, err := codec.Value("movies", spec)

	if err != nil {
		test.Fatal(err)
	}

	state, err := codec.secrets.stateHashes(config, config)

	if err != nil {
		test.Fatal(err)
	}

	unset, err := codec.secrets.nullSecrets(config)

	if err != nil {
		test.Fatal(err)
	}

	if removed := codec.secrets.removed(unset, state); len(removed) != 1 || !removed[0].Equal(testPasswordPath) {
		test.Errorf("unset password: removed = %v, want the password", removed)
	}

	if removed := codec.secrets.removed(config, state); len(removed) != 0 {
		test.Errorf("set password: removed = %v, want none", removed)
	}

	if removed := codec.secrets.removed(unset, tftypes.NewValue(state.Type(), nil)); len(removed) != 0 {
		test.Errorf("create: removed = %v, want none", removed)
	}
}

func TestCodecSecretsUnplanned(test *testing.T) {
	var spec CaptureSpec[testConfig, testResource] = testCaptureSpec

	codec, err := newCodec[captureDocument[testConfig, testResource]](testNamedConnector, captureType[testResource]())

	if err != nil {
		test.Fatal(err)
	}

	spec.Endpoint.Connector.Config.Password = "secret"

	config, err := codec.Value("movies", spec)

	if err != nil {
		test.Fatal(err)
	}

	unset, err := codec.secrets.nullSecrets(config)

	if err != nil {
		test.Fatal(err)
	}

	empty, err := withValue(config, testPasswordPath, "")

	if err != nil {
		test.Fatal(err)
	}

	unknownHash, err := withValue(unset, testPasswordHashPath, tftypes.UnknownValue)

	if err != nil {
		test.Fatal(err)
	}

	nullHash, err := withValue(unset, testPasswordHashPath, nil)

	if err != nil {
		test.Fatal(err)
	}

	expectations := map[string]struct {
		plan    tftypes.Value
		config  tftypes.Value
		flagged bool
	}{
		"null at apply":                 {unknownHash, unset, true},
		"empty at apply":                {unknownHash, empty, true},
		"set at apply":                  {unknownHash, config, false},
		"removal the plan warned about": {nullHash, unset, false},
	}

	for name, expected := range expectations {
		var unplanned []*tftypes.AttributePath = codec.secrets.unplanned(expected.plan, expected.config)

		if expected.flagged && (len(unplanned) != 1 || !unplanned[0].Equal(testPasswordPath)) {
			test.Errorf("%s: unplanned = %v, want the password", name, unplanned)
		}

		if !expected.flagged && len(unplanned) != 0 {
			test.Errorf("%s: unplanned = %v, want none", name, unplanned)
		}
	}
}

func TestCodecSecretPath(test *testing.T) {
	codec, err := newCodec[captureDocument[testConfig, testResource]](testNamedConnector, captureType[testResource]())

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
		secret, isHash := codec.secrets.secretPath(expected.path)

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
