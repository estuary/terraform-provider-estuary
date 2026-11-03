package builder

import (
	"context"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// testPrivate is an in-memory privateData.
type testPrivate map[string][]byte

func (private testPrivate) GetKey(_ context.Context, key string) ([]byte, diag.Diagnostics) {
	return private[key], nil
}

func (private testPrivate) SetKey(_ context.Context, key string, value []byte) diag.Diagnostics {
	private[key] = value

	return nil
}

// sealedCapture returns a live capture whose password Estuary encrypted as encrypted.
func sealedCapture(encrypted string) map[string]any {
	return map[string]any{
		"endpoint": map[string]any{
			"connector": map[string]any{
				"image":  testNamedConnector.Image,
				"config": map[string]any{"host": "db.example.com", "password_sops": encrypted, "sops": map[string]any{"mac": "<encrypted>"}},
			},
		},
		"bindings": []any{},
	}
}

func TestFingerprintRoundTrip(test *testing.T) {
	var ctx context.Context = context.Background()
	var private testPrivate = testPrivate{}

	if diagnostics := storeFingerprint(ctx, private, "sha256:abc"); diagnostics.HasError() {
		test.Fatal(diagnostics)
	}

	if stored, diagnostics := storedFingerprint(ctx, private); diagnostics.HasError() || stored != "sha256:abc" {
		test.Errorf("stored fingerprint = %q (%v), want sha256:abc", stored, diagnostics)
	}

	if stored, diagnostics := storedFingerprint(ctx, testPrivate{}); diagnostics.HasError() || stored != "" {
		test.Errorf("fingerprint of empty private state = %q (%v), want empty", stored, diagnostics)
	}

	if stored, diagnostics := storedFingerprint(ctx, nil); diagnostics.HasError() || stored != "" {
		test.Errorf("fingerprint without private state = %q (%v), want empty", stored, diagnostics)
	}
}

func TestReadDetectsSecretsChangedOutsideTerraform(test *testing.T) {
	var ctx context.Context = context.Background()
	var published string = catalog.SecretsFingerprint(sealedCapture("ENC[AES256_GCM,data:published,type:str]"))
	var rotated string = catalog.SecretsFingerprint(sealedCapture("ENC[AES256_GCM,data:rotated,type:str]"))

	hash, err := hashSecret("secret")

	if err != nil {
		test.Fatal(err)
	}

	expectations := map[string]struct {
		live      string
		stored    string
		rotates   bool
		unmanaged bool
		drift     bool
		want      string
	}{
		"keeps unchanged secrets":              {"published", published, false, false, false, published},
		"detects secrets entered again":        {"rotated", published, false, false, true, published},
		"takes the live secrets without any":   {"rotated", "", false, false, false, rotated},
		"lets connectors rotate their secrets": {"rotated", published, true, false, false, rotated},
		"takes secrets Terraform doesn't set":  {"rotated", published, false, true, false, rotated},
	}

	for name, expected := range expectations {
		entry := newTestCaptureEntry(&testBackend{live: sealedCapture("ENC[AES256_GCM,data:" + expected.live + ",type:str]")})

		entry.connector.RotatesSecrets = expected.rotates

		stateValue, err := entry.codec.Value("movies", testCaptureSpec)

		if err == nil && !expected.unmanaged {
			stateValue, err = withValue(stateValue, testPasswordHashPath, hash)
		}

		if err != nil {
			test.Fatal(err)
		}

		var state tfsdk.State = tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}
		var readResp resource.ReadResponse = resource.ReadResponse{State: state}

		fingerprint, diagnostics := entry.readState(ctx, "movies", expected.stored, resource.ReadRequest{State: state}, &readResp)

		if diagnostics.HasError() {
			test.Fatalf("%s: read: %v", name, diagnostics)
		}

		var warned bool = len(diagnostics.Warnings()) == 1 && strings.HasPrefix(diagnostics.Warnings()[0].Summary(), "Secrets changed outside Terraform")

		if warned != expected.drift || fingerprint != expected.want {
			test.Errorf("%s: warned %v, fingerprint %q, want warned %v, fingerprint %q", name, warned, fingerprint, expected.drift, expected.want)
		}

		refreshedHash, err := attributeValue(readResp.State.Raw, testPasswordHashPath)

		if err != nil {
			test.Fatal(err)
		}

		if refreshedHash.IsNull() != (expected.drift || expected.unmanaged) {
			test.Errorf("%s: refreshed password_hash = %v, want null %v so the plan publishes the configured secret", name, refreshedHash, expected.drift)
		}

		var configured CaptureSpec[testConfig, testResource] = testCaptureSpec

		if !expected.unmanaged {
			configured.Endpoint.Connector.Config.Password = "secret"
		}

		config, err := entry.codec.Value("movies", configured)

		if err != nil {
			test.Fatal(err)
		}

		planned, _, err := entry.codec.secrets.planHashes(readResp.State.Raw, config, readResp.State.Raw)

		if err != nil {
			test.Fatal(err)
		}

		if republishes := entry.codec.secrets.changed(planned, readResp.State.Raw); republishes != expected.drift {
			test.Errorf("%s: apply publishes the configured secrets %v, want %v", name, republishes, expected.drift)
		}
	}
}

func TestPublishedFingerprint(test *testing.T) {
	var ctx context.Context = context.Background()
	var live map[string]any = sealedCapture("ENC[AES256_GCM,data:published,type:str]")
	var configured CaptureSpec[testConfig, testResource] = testCaptureSpec

	configured.Endpoint.Connector.Config.Password = "secret"

	expectations := map[string]struct {
		document map[string]any
		spec     CaptureSpec[testConfig, testResource]
		reads    int
		want     string
	}{
		"keeps the sealed secrets":       {live, configured, 0, catalog.SecretsFingerprint(live)},
		"reads secrets Estuary encrypts": {map[string]any{"endpoint": map[string]any{"connector": map[string]any{"config": map[string]any{"password": "secret"}}}}, configured, 1, catalog.SecretsFingerprint(live)},
		"skips entries without secrets":  {map[string]any{"endpoint": map[string]any{}}, testCaptureSpec, 0, ""},
	}

	for name, expected := range expectations {
		var backend *testBackend = &testBackend{live: live}

		entry := newTestCaptureEntry(backend)

		planned, err := entry.codec.Value("movies", expected.spec)

		if err != nil {
			test.Fatal(err)
		}

		fingerprint, diagnostics := entry.publishedFingerprint(ctx, "acmeCo/movies/source-test", expected.document, planned)

		if diagnostics.HasError() || fingerprint != expected.want || len(backend.reads) != expected.reads {
			test.Errorf("%s: fingerprint %q after %d reads (%v), want %q after %d", name, fingerprint, len(backend.reads), diagnostics, expected.want, expected.reads)
		}
	}
}
