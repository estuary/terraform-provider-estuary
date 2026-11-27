package builder

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/googleplay"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var googlePlayConnector Connector = Connector{
	TypeName:      "source_google_play",
	Image:         googleplay.Image,
	ConnectorName: googleplay.ConnectorName,
}

// googlePlayLive is a Google Play capture as the dashboard publishes it: a sealed service account key and discovered
// bindings with their resource paths.
const googlePlayLive string = `
	autoDiscover:
		addNewBindings: true
		evolveIncompatibleCollections: true
	endpoint:
		connector:
			image: ghcr.io/estuary/source-google-play:v1
			config:
				bucket: pubsite_prod_1234567890123456789
				start_date: "2024-01-01T00:00:00Z"
				credentials:
					credentials_title: Google Service Account
					service_account_sops: ENC[AES256_GCM,data:abc,type:str]
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
					path: [installs]
				name: installs
				interval: PT1H
			target: acmeCo/google-play/installs
			backfill: 1
		- resource:
				_meta:
					path: [reviews]
				name: reviews
				interval: PT1H
			target: acmeCo/google-play/reviews
	shards:
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

func googlePlaySpec(config googleplay.Config, bindings ...capture.Binding[googleplay.Resource]) CaptureSpec[googleplay.Config, googleplay.Resource] {
	return CaptureSpec[googleplay.Config, googleplay.Resource]{
		AutoDiscover: &capture.AutoDiscover{AddNewBindings: true, EvolveIncompatibleCollections: true},
		Endpoint:     endpoint.ConnectorSpec[googleplay.Config]{Connector: endpoint.Connector[googleplay.Config]{Image: googleplay.Image, Config: config}},
		Bindings:     bindings,
	}
}

func TestGooglePlaySamplesRoundTrip(test *testing.T) {
	assertCaptureSamplesRoundTrip[googleplay.Config, googleplay.Resource](test, googlePlayConnector, "../../catalog/capture/googleplay/testdata")
}

func TestGooglePlayImportPlansNoChanges(test *testing.T) {
	var config googleplay.Config = googleplay.Config{
		Bucket:      "pubsite_prod_1234567890123456789",
		StartDate:   "2024-01-01T00:00:00Z",
		Credentials: googleplay.Credentials{Title: googleplay.CredentialsTitleServiceAccount},
	}

	entry := newConnectorCapture[googleplay.Config, googleplay.Resource](test, googlePlayConnector, googlePlayLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/apps/source-google-play", false)

	entry.assertPlansNoChanges(test, "acmeCo/apps", googlePlaySpec(config), state)

	entry.assertAppliesWithoutPublishing(test, "acmeCo/apps", googlePlaySpec(config,
		capture.Binding[googleplay.Resource]{Resource: googleplay.Resource{Name: "installs", Interval: "PT1H"}, Target: "acmeCo/google-play/installs"},
		capture.Binding[googleplay.Resource]{Resource: googleplay.Resource{Name: "reviews", Interval: "PT1H"}, Target: "acmeCo/google-play/reviews"},
	), state)
}

func TestGooglePlaySecretsAreWriteOnly(test *testing.T) {
	codec, err := newCodec[captureDocument[googleplay.Config, googleplay.Resource]](googlePlayConnector, captureType[googleplay.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	config, err := codec.Value("apps", googlePlaySpec(googleplay.Config{
		Bucket:      "pubsite_prod_1234567890123456789",
		Credentials: googleplay.Credentials{Title: googleplay.CredentialsTitleServiceAccount, ServiceAccount: `{"type": "service_account"}`},
	}))

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, codec.secrets, config, configAttributePath("credentials", "service_account"))

	if path := configAttributePath("credentials", "credentials_title"); codec.secrets.attribute(path) {
		test.Errorf("%s is a secret, want it readable in state", path)
	}
}
