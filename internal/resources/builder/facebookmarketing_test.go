package builder

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/facebookmarketing"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var facebookMarketingConnector Connector = Connector{
	TypeName:      "source_facebook_marketing",
	Image:         facebookmarketing.Image,
	ConnectorName: facebookmarketing.ConnectorName,
}

// facebookMarketingLive is a Facebook Marketing capture created through the dashboard login: the sealed credentials
// of Estuary's Meta app, explicit defaults, a custom insight with its null action breakdowns, and discovered bindings
// with their resource paths.
const facebookMarketingLive string = `
	autoDiscover:
		addNewBindings: true
		evolveIncompatibleCollections: true
	endpoint:
		connector:
			image: ghcr.io/estuary/source-facebook-marketing-native:v1
			config:
				account_ids: "123456789,987654321"
				start_date: "2024-01-01T00:00:00Z"
				insights_lookback_window: 28
				custom_insights:
					- name: spend_by_age
						level: campaign
						fields: account_id,campaign_id,spend
						breakdowns: age
						action_breakdowns: null
						insights_lookback_window: 28
				advanced:
					include_deleted: false
					fetch_thumbnail_images: false
					feature_flags: ""
				credentials:
					credentials_title: OAuth Credentials
					client_id_sops: ENC[AES256_GCM,data:abc,type:str]
					client_secret_sops: ENC[AES256_GCM,data:def,type:str]
					access_token_sops: ENC[AES256_GCM,data:ghi,type:str]
				sops:
					gcp_kms:
						- resource_id: projects/example/locations/us-central1/keyRings/example/cryptoKeys/example
							created_at: "2026-01-01T00:00:00Z"
							enc: <encrypted>
					lastmodified: "2026-01-01T00:00:00Z"
					mac: ENC[AES256_GCM,data:jkl,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				_meta:
					path: [campaigns]
				name: campaigns
				interval: PT1H
			target: acmeCo/facebook/campaigns
			backfill: 1
		- resource:
				_meta:
					path: [spend_by_age]
				name: spend_by_age
				interval: PT1H
			target: acmeCo/facebook/spend_by_age
	shards:
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

func facebookMarketingSpec(config facebookmarketing.Config, bindings ...capture.Binding[facebookmarketing.Resource]) CaptureSpec[facebookmarketing.Config, facebookmarketing.Resource] {
	return CaptureSpec[facebookmarketing.Config, facebookmarketing.Resource]{
		AutoDiscover: &capture.AutoDiscover{AddNewBindings: true, EvolveIncompatibleCollections: true},
		Endpoint:     endpoint.ConnectorSpec[facebookmarketing.Config]{Connector: endpoint.Connector[facebookmarketing.Config]{Image: facebookmarketing.Image, Config: config}},
		Bindings:     bindings,
	}
}

func TestFacebookMarketingSamplesRoundTrip(test *testing.T) {
	assertCaptureSamplesRoundTrip[facebookmarketing.Config, facebookmarketing.Resource](test, facebookMarketingConnector, "../../catalog/capture/facebookmarketing/testdata")
}

// A capture created through the dashboard login imports with its OAuth secrets left out of the configuration, which
// then plans no changes and applies without publishing.
func TestFacebookMarketingImportOfADashboardLoginPlansNoChanges(test *testing.T) {
	var config facebookmarketing.Config = facebookmarketing.Config{
		AccountIDs:             "123456789,987654321",
		StartDate:              "2024-01-01T00:00:00Z",
		InsightsLookbackWindow: 28,
		CustomInsights: []facebookmarketing.CustomInsight{{
			Name:                   "spend_by_age",
			Level:                  facebookmarketing.InsightLevelCampaign,
			Fields:                 "account_id,campaign_id,spend",
			Breakdowns:             "age",
			InsightsLookbackWindow: 28,
		}},
		Advanced:    &facebookmarketing.Advanced{},
		Credentials: facebookmarketing.Credentials{Title: native.CredentialsTitleOAuth},
	}

	entry := newConnectorCapture[facebookmarketing.Config, facebookmarketing.Resource](test, facebookMarketingConnector, facebookMarketingLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/ads/source-facebook-marketing-native", false)

	entry.assertPlansNoChanges(test, "acmeCo/ads", facebookMarketingSpec(config), state)

	entry.assertAppliesWithoutPublishing(test, "acmeCo/ads", facebookMarketingSpec(config,
		capture.Binding[facebookmarketing.Resource]{Resource: facebookmarketing.Resource{Name: "campaigns", Interval: "PT1H"}, Target: "acmeCo/facebook/campaigns"},
		capture.Binding[facebookmarketing.Resource]{Resource: facebookmarketing.Resource{Name: "spend_by_age", Interval: "PT1H"}, Target: "acmeCo/facebook/spend_by_age"},
	), state)
}

func TestFacebookMarketingSecretsAreWriteOnly(test *testing.T) {
	codec, err := newCodec[captureDocument[facebookmarketing.Config, facebookmarketing.Resource]](facebookMarketingConnector, captureType[facebookmarketing.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	config, err := codec.Value("ads", facebookMarketingSpec(facebookmarketing.Config{
		AccountIDs: "123456789",
		Credentials: facebookmarketing.Credentials{
			Title:       native.CredentialsTitleOAuth,
			OAuthClient: native.OAuthClient{ClientID: "<app-id>", ClientSecret: "<app-secret>"},
			AccessToken: "<access-token>",
		},
	}))

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, codec.secrets, config,
		configAttributePath("credentials", "client_id"),
		configAttributePath("credentials", "client_secret"),
		configAttributePath("credentials", "access_token"),
	)

	if path := configAttributePath("credentials", "credentials_title"); codec.secrets.attribute(path) {
		test.Errorf("%s is a secret, want it readable in state", path)
	}
}
