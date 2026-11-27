package builder

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/httpingest"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var httpIngestConnector Connector = Connector{
	TypeName:      "source_http_ingest",
	Image:         httpingest.Image,
	ConnectorName: httpingest.ConnectorName,
}

// httpIngestLive is an HTTP Ingest capture as the dashboard publishes it with an auth token: the sealed token, the
// empty CORS list a token requires, two paths with their bindings and resource paths, and a signature check.
const httpIngestLive string = `
	endpoint:
		connector:
			image: ghcr.io/estuary/source-http-ingest:v1
			config:
				paths:
					- /orders
					- /vendors/{vendorId}/products
				allowedCorsOrigins: []
				requireAuthToken_sops: ENC[AES256_GCM,data:abc,type:str]
				signatureConfig:
					provider: zoom
					publicKey_sops: ENC[AES256_GCM,data:def,type:str]
					maxSignatureAge: 300
				sops:
					gcp_kms:
						- resource_id: projects/example/locations/us-central1/keyRings/example/cryptoKeys/example
							created_at: "2026-01-01T00:00:00Z"
							enc: <encrypted>
					lastmodified: "2026-01-01T00:00:00Z"
					mac: ENC[AES256_GCM,data:ghi,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				_meta:
					path: [/orders]
				path: /orders
				stream: /orders
			target: acmeCo/webhooks/orders
			backfill: 1
		- resource:
				_meta:
					path: ["/vendors/{vendorId}/products"]
				path: /vendors/{vendorId}/products
				stream: /vendors/{vendorId}/products
				idFromHeader: X-Request-Id
			target: acmeCo/webhooks/vendor-products
	shards:
		disable: true
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

// httpIngestOpenLive is an HTTP Ingest capture without an auth token, whose binding predates resource paths.
const httpIngestOpenLive string = `
	endpoint:
		connector:
			image: ghcr.io/estuary/source-http-ingest:v1
			config:
				paths: []
				sops:
					mac: ENC[AES256_GCM,data:ghi,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				stream: events
				path: /events
			target: acmeCo/webhooks/events
	expectPubId: "11:22:33:44:55:66:77:88"
`

func httpIngestSpec(config httpingest.Config, shards catalog.Shards, bindings ...capture.Binding[httpingest.Resource]) CaptureSpec[httpingest.Config, httpingest.Resource] {
	return CaptureSpec[httpingest.Config, httpingest.Resource]{
		Endpoint: endpoint.ConnectorSpec[httpingest.Config]{Connector: endpoint.Connector[httpingest.Config]{Image: httpingest.Image, Config: config}},
		Bindings: bindings,
		Shards:   shards,
	}
}

func TestHTTPIngestSamplesRoundTrip(test *testing.T) {
	assertCaptureSamplesRoundTrip[httpingest.Config, httpingest.Resource](test, httpIngestConnector, "../../catalog/capture/httpingest/testdata")
}

func TestHTTPIngestImportPlansNoChanges(test *testing.T) {
	var config httpingest.Config = httpingest.Config{
		Paths:              &[]string{"/orders", "/vendors/{vendorId}/products"},
		AllowedCORSOrigins: &[]string{},
		SignatureConfig:    &httpingest.SignatureConfig{Provider: httpingest.SignatureProviderZoom, MaxSignatureAge: 300},
	}
	var disabled catalog.Shards = catalog.Shards{"disable": true}

	entry := newConnectorCapture[httpingest.Config, httpingest.Resource](test, httpIngestConnector, httpIngestLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/webhooks/source-http-ingest", false)

	entry.assertPlansNoChanges(test, "acmeCo/webhooks", httpIngestSpec(config, disabled), state)

	entry.assertAppliesWithoutPublishing(test, "acmeCo/webhooks", httpIngestSpec(config, disabled,
		capture.Binding[httpingest.Resource]{Resource: httpingest.Resource{Stream: "/orders", Path: "/orders"}, Target: "acmeCo/webhooks/orders"},
		capture.Binding[httpingest.Resource]{
			Resource: httpingest.Resource{Stream: "/vendors/{vendorId}/products", Path: "/vendors/{vendorId}/products", IDFromHeader: "X-Request-Id"},
			Target:   "acmeCo/webhooks/vendor-products",
		},
	), state)

	var openConfig httpingest.Config = httpingest.Config{Paths: &[]string{}}

	open := newConnectorCapture[httpingest.Config, httpingest.Resource](test, httpIngestConnector, httpIngestOpenLive)

	state = open.importState(test, "acmeCo/webhooks/source-http-ingest", false)

	open.assertPlansNoChanges(test, "acmeCo/webhooks", httpIngestSpec(openConfig, nil), state)

	open.assertAppliesWithoutPublishing(test, "acmeCo/webhooks", httpIngestSpec(openConfig, nil,
		capture.Binding[httpingest.Resource]{Resource: httpingest.Resource{Stream: "events", Path: "/events"}, Target: "acmeCo/webhooks/events"},
	), state)
}

func TestHTTPIngestSecretsAreWriteOnly(test *testing.T) {
	codec, err := newCodec[captureDocument[httpingest.Config, httpingest.Resource]](httpIngestConnector, captureType[httpingest.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	config, err := codec.Value("webhooks", httpIngestSpec(httpingest.Config{
		Paths:            &[]string{"/orders"},
		RequireAuthToken: "<token>",
		SignatureConfig:  &httpingest.SignatureConfig{Provider: httpingest.SignatureProviderZoom, PublicKey: "<secret-token>"},
	}, nil))

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, codec.secrets, config,
		configAttributePath("require_auth_token"),
		configAttributePath("signature_config", "public_key"),
	)
}
