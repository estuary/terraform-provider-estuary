package builder

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/stripe"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var stripeConnector Connector = Connector{
	TypeName:      "source_stripe",
	Image:         stripe.Image,
	ConnectorName: stripe.ConnectorName,
}

// stripeLive is a Stripe capture as the dashboard publishes it: a sealed API key, explicit defaults, discovered
// bindings with their resource paths, one of them backfilled on a schedule, and a runtime shard flag.
const stripeLive string = `
	autoDiscover:
		addNewBindings: true
		evolveIncompatibleCollections: true
	endpoint:
		connector:
			image: ghcr.io/estuary/source-stripe-native:v1
			config:
				credentials:
					credentials_title: Private App Credentials
					access_token_sops: ENC[AES256_GCM,data:abc,type:str]
				start_date: "2024-01-01T00:00:00Z"
				capture_connected_accounts: false
				advanced:
					api_version: null
					incremental_window_size: P1D
				sops:
					gcp_kms:
						- resource_id: projects/example/locations/us-central1/keyRings/example/cryptoKeys/example
							created_at: "2026-01-01T00:00:00Z"
							enc: <encrypted>
					lastmodified: "2026-01-01T00:00:00Z"
					mac: ENC[AES256_GCM,data:def,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				_meta:
					path: [Charges]
				name: Charges
				interval: PT5M
			target: acmeCo/stripe/Charges
			backfill: 1
		- resource:
				_meta:
					path: [Accounts]
				name: Accounts
				interval: PT5M
				schedule: 0 0 * * *
			target: acmeCo/stripe/Accounts
	shards:
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

func stripeSpec(config stripe.Config, bindings ...capture.Binding[stripe.Resource]) CaptureSpec[stripe.Config, stripe.Resource] {
	return CaptureSpec[stripe.Config, stripe.Resource]{
		AutoDiscover: &capture.AutoDiscover{AddNewBindings: true, EvolveIncompatibleCollections: true},
		Endpoint:     endpoint.ConnectorSpec[stripe.Config]{Connector: endpoint.Connector[stripe.Config]{Image: stripe.Image, Config: config}},
		Bindings:     bindings,
	}
}

func TestStripeSamplesRoundTrip(test *testing.T) {
	assertCaptureSamplesRoundTrip[stripe.Config, stripe.Resource](test, stripeConnector, "../../catalog/capture/stripe/testdata")
}

func TestStripeImportPlansNoChanges(test *testing.T) {
	var config stripe.Config = stripe.Config{
		Credentials: stripe.Credentials{Title: native.CredentialsTitlePrivateApp},
		StartDate:   "2024-01-01T00:00:00Z",
		Advanced:    &stripe.Advanced{IncrementalWindowSize: "P1D"},
	}

	entry := newConnectorCapture[stripe.Config, stripe.Resource](test, stripeConnector, stripeLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/payments/source-stripe-native", false)

	entry.assertPlansNoChanges(test, "acmeCo/payments", stripeSpec(config), state)

	entry.assertAppliesWithoutPublishing(test, "acmeCo/payments", stripeSpec(config,
		capture.Binding[stripe.Resource]{Resource: stripe.Resource{Resource: native.Resource{Name: "Charges", Interval: "PT5M"}}, Target: "acmeCo/stripe/Charges"},
		capture.Binding[stripe.Resource]{Resource: stripe.Resource{Resource: native.Resource{Name: "Accounts", Interval: "PT5M"}, Schedule: "0 0 * * *"}, Target: "acmeCo/stripe/Accounts"},
	), state)
}

func TestStripeSecretsAreWriteOnly(test *testing.T) {
	codec, err := newCodec[captureDocument[stripe.Config, stripe.Resource]](stripeConnector, captureType[stripe.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	config, err := codec.Value("payments", stripeSpec(stripe.Config{
		Credentials: stripe.Credentials{Title: native.CredentialsTitlePrivateApp, AccessToken: "<api-key>"},
	}))

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, codec.secrets, config, configAttributePath("credentials", "access_token"))

	if path := configAttributePath("credentials", "credentials_title"); codec.secrets.attribute(path) {
		test.Errorf("%s is a secret, want it readable in state", path)
	}
}
