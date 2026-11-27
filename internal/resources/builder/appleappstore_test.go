package builder

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/appleappstore"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var appleAppStoreConnector Connector = Connector{
	TypeName:      "source_apple_app_store",
	Image:         appleappstore.Image,
	ConnectorName: appleappstore.ConnectorName,
}

// appleAppStoreLive is an Apple App Store capture as the dashboard publishes it: a sealed API key, with the hidden
// access token the form leaves empty, an empty app list, and discovered bindings with their resource paths.
const appleAppStoreLive string = `
	autoDiscover:
		addNewBindings: true
		evolveIncompatibleCollections: true
	endpoint:
		connector:
			image: ghcr.io/estuary/source-apple-app-store:v1
			config:
				credentials:
					credentials_title: Private App Credentials
					access_token_sops: ENC[AES256_GCM,data:abc,type:str]
					private_key_sops: ENC[AES256_GCM,data:def,type:str]
					key_id_sops: ENC[AES256_GCM,data:ghi,type:str]
					issuer_id_sops: ENC[AES256_GCM,data:jkl,type:str]
				app_ids: []
				sops:
					gcp_kms:
						- resource_id: projects/example/locations/us-central1/keyRings/example/cryptoKeys/example
							created_at: "2026-01-01T00:00:00Z"
							enc: <encrypted>
					lastmodified: "2026-01-01T00:00:00Z"
					mac: ENC[AES256_GCM,data:mno,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				_meta:
					path: [app_reviews]
				name: app_reviews
				interval: PT1H
			target: acmeCo/app-store/app_reviews
			backfill: 1
		- resource:
				_meta:
					path: [app_sessions]
				name: app_sessions
				interval: PT1H
			target: acmeCo/app-store/app_sessions
	shards:
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

func appleAppStoreSpec(config appleappstore.Config, bindings ...capture.Binding[appleappstore.Resource]) CaptureSpec[appleappstore.Config, appleappstore.Resource] {
	return CaptureSpec[appleappstore.Config, appleappstore.Resource]{
		AutoDiscover: &capture.AutoDiscover{AddNewBindings: true, EvolveIncompatibleCollections: true},
		Endpoint:     endpoint.ConnectorSpec[appleappstore.Config]{Connector: endpoint.Connector[appleappstore.Config]{Image: appleappstore.Image, Config: config}},
		Bindings:     bindings,
	}
}

func TestAppleAppStoreSamplesRoundTrip(test *testing.T) {
	assertCaptureSamplesRoundTrip[appleappstore.Config, appleappstore.Resource](test, appleAppStoreConnector, "../../catalog/capture/appleappstore/testdata")
}

func TestAppleAppStoreImportPlansNoChanges(test *testing.T) {
	var config appleappstore.Config = appleappstore.Config{
		Credentials: appleappstore.Credentials{Title: native.CredentialsTitlePrivateApp},
	}

	entry := newConnectorCapture[appleappstore.Config, appleappstore.Resource](test, appleAppStoreConnector, appleAppStoreLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/apps/source-apple-app-store", false)

	entry.assertPlansNoChanges(test, "acmeCo/apps", appleAppStoreSpec(config), state)

	entry.assertAppliesWithoutPublishing(test, "acmeCo/apps", appleAppStoreSpec(config,
		capture.Binding[appleappstore.Resource]{Resource: appleappstore.Resource{Name: "app_reviews", Interval: "PT1H"}, Target: "acmeCo/app-store/app_reviews"},
		capture.Binding[appleappstore.Resource]{Resource: appleappstore.Resource{Name: "app_sessions", Interval: "PT1H"}, Target: "acmeCo/app-store/app_sessions"},
	), state)
}

func TestAppleAppStoreSecretsAreWriteOnly(test *testing.T) {
	codec, err := newCodec[captureDocument[appleappstore.Config, appleappstore.Resource]](appleAppStoreConnector, captureType[appleappstore.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	config, err := codec.Value("apps", appleAppStoreSpec(appleappstore.Config{
		Credentials: appleappstore.Credentials{
			Title:       native.CredentialsTitlePrivateApp,
			PrivateKey:  "<private-key>",
			KeyID:       "<key-id>",
			IssuerID:    "<issuer-id>",
			AccessToken: "<access-token>",
		},
	}))

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, codec.secrets, config,
		configAttributePath("credentials", "private_key"),
		configAttributePath("credentials", "key_id"),
		configAttributePath("credentials", "issuer_id"),
		configAttributePath("credentials", "access_token"),
	)

	if path := configAttributePath("credentials", "credentials_title"); codec.secrets.attribute(path) {
		test.Errorf("%s is a secret, want it readable in state", path)
	}
}
