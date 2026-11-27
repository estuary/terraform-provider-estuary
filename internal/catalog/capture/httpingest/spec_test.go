package httpingest

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunCaptureSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, Resource](test)
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, SignatureProviders, SignatureAlgorithms, SignatureEncodings)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		paths     []string
		cors      *[]string
		token     string
		signature *SignatureConfig
	}{
		"acmeCo/webhooks/open":             {paths: []string{"/webhook-data"}},
		"acmeCo/webhooks/auth-token":       {paths: []string{"/orders", "/vendors/{vendorId}/products"}, cors: &[]string{}, token: "<token>"},
		"acmeCo/webhooks/zoom-signature":   {paths: []string{"/zoom"}, signature: &SignatureConfig{Provider: SignatureProviderZoom, PublicKey: "<secret-token>"}},
		"acmeCo/webhooks/twilio-signature": {paths: []string{"/sendgrid"}, signature: &SignatureConfig{Provider: SignatureProviderTwilio, MaxSignatureAge: 600}},
		"acmeCo/webhooks/knock-signature":  {paths: []string{"/knock"}, signature: &SignatureConfig{Provider: SignatureProviderKnock, PublicKey: "<signing-key>"}},
		"acmeCo/webhooks/no-signature":     {paths: []string{}, signature: &SignatureConfig{Provider: SignatureProviderNone}},
	}

	if len(specs) != len(expectations) {
		test.Fatalf("got %d captures, want %d", len(specs), len(expectations))
	}

	for name, expected := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			if !found {
				test.Fatal("capture missing from sample")
			}

			var config Config = spec.Endpoint.Connector.Config

			if config.Paths == nil || !slices.Equal(*config.Paths, expected.paths) || config.RequireAuthToken != expected.token {
				test.Errorf("paths = %v, requireAuthToken = %q, want %q and %q", config.Paths, config.RequireAuthToken, expected.paths, expected.token)
			}

			if !reflect.DeepEqual(config.AllowedCORSOrigins, expected.cors) {
				test.Errorf("allowedCorsOrigins = %v, want %v, an empty list being kept apart from an unset one", config.AllowedCORSOrigins, expected.cors)
			}

			if expected.signature != nil && expected.signature.Provider == SignatureProviderTwilio && config.SignatureConfig != nil {
				expected.signature.PublicKey = config.SignatureConfig.PublicKey

				if !strings.HasPrefix(config.SignatureConfig.PublicKey, "-----BEGIN PUBLIC KEY-----\n") {
					test.Errorf("publicKey = %q, want a multiline PEM key", config.SignatureConfig.PublicKey)
				}
			}

			if !reflect.DeepEqual(config.SignatureConfig, expected.signature) {
				test.Errorf("signatureConfig = %+v, want %+v", config.SignatureConfig, expected.signature)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/webhooks/all-options"]

	var expectedConfig Config = Config{
		Paths:              &[]string{"/github", "/shopify/orders"},
		AllowedCORSOrigins: &[]string{"https://app.acme.example", "https://admin.acme.example"},
		RequireAuthToken:   "<token>",
		SignatureConfig: &SignatureConfig{
			Provider:              SignatureProviderCustom,
			PublicKey:             "<shared-secret>",
			MaxSignatureAge:       120,
			Algorithm:             SignatureAlgorithmHMACSHA256,
			SignatureHeader:       "X-Signature",
			SignatureEncoding:     SignatureEncodingBase64,
			SigningStringTemplate: "{TIMESTAMP}:{PAYLOAD}",
			TimestampHeader:       "X-Timestamp",
		},
	}

	if config := spec.Endpoint.Connector.Config; !reflect.DeepEqual(config, expectedConfig) {
		test.Errorf("config = %+v, want %+v", config, expectedConfig)
	}

	expectedBindings := []Binding{
		{Resource: Resource{Stream: "/github", Path: "/github", IDFromHeader: "X-GitHub-Delivery"}, Target: "acmeCo/webhooks/github"},
		{Resource: Resource{Stream: "/shopify/orders", Path: "/shopify/orders"}, Target: "acmeCo/webhooks/shopify-orders", Backfill: 1},
		{Resource: Resource{Stream: "legacy"}, Target: "acmeCo/webhooks/legacy", Disable: true},
	}

	if !reflect.DeepEqual(spec.Bindings, expectedBindings) {
		test.Errorf("bindings = %+v, want %+v", spec.Bindings, expectedBindings)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var providers *spectest.EnumSamples[SignatureProvider] = spectest.NewEnumSamples(test, "signatureConfig provider", SignatureProviders)
	var sampled map[string]bool = map[string]bool{}

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			sampled["auth token "+map[bool]string{true: "set", false: "unset"}[config.RequireAuthToken != ""]] = true

			if config.Paths != nil && len(*config.Paths) > 1 {
				sampled["several paths"] = true
			}

			if config.Paths != nil {
				for _, path := range *config.Paths {
					if !strings.HasPrefix(path, "/") || len(path) < 2 {
						test.Errorf("%s: path %q must start with / and name something", name, path)
					}
				}
			}

			if config.RequireAuthToken != "" && (config.AllowedCORSOrigins == nil || slices.Contains(*config.AllowedCORSOrigins, "*")) {
				test.Errorf("%s: allowedCorsOrigins %v must be set without * when an auth token is", name, config.AllowedCORSOrigins)
			}

			if signature := config.SignatureConfig; signature != nil {
				providers.Add(name, signature.Provider)

				var custom bool = signature.Algorithm != "" && signature.SignatureHeader != "" && signature.SigningStringTemplate != ""

				if (signature.Provider == SignatureProviderNone) == (signature.PublicKey != "") || (signature.Provider == SignatureProviderCustom) != custom {
					test.Errorf("%s: signatureConfig %+v must set the fields its provider requires and no custom field otherwise", name, signature)
				}
			}
		}
	}

	providers.AssertAllSampled()

	for _, shape := range []string{"auth token set", "auth token unset", "several paths"} {
		if !sampled[shape] {
			test.Errorf("no sample with %s", shape)
		}
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"secret isn't the key of the signature key": {
			Key: "secret",
			Content: `
				captures:
					acmeCo/webhooks/secret:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-http-ingest:v1
								config:
									signatureConfig:
										provider: zoom
										secret: <secret-token>
						bindings: []
			`,
		},
		"snake case auth token": {
			Key: "require_auth_token",
			Content: `
				captures:
					acmeCo/webhooks/snake-case:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-http-ingest:v1
								config:
									require_auth_token: <token>
						bindings: []
			`,
		},
		"name is the resource key of other connectors": {
			Key: "name",
			Content: `
				captures:
					acmeCo/webhooks/name:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-http-ingest:v1
								config:
									paths:
										- /orders
						bindings:
							- resource:
									stream: /orders
									name: orders
								target: acmeCo/webhooks/orders
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}

func TestResourcePath(test *testing.T) {
	if path := (Resource{Stream: "/orders", Path: "/v2/orders"}).ResourcePath(); !slices.Equal(path, []string{"/orders"}) {
		test.Errorf("resource path = %q, want [/orders]", path)
	}
}
