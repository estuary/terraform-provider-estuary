package facebookmarketing

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunCaptureSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, native.SnapshotResource[Resource]](test)
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, CredentialsTitles, InsightLevels)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		accountIDs  string
		credentials Credentials
	}{
		"acmeCo/facebook/access-token": {"123456789", Credentials{
			Title:       native.CredentialsTitleOAuth,
			OAuthClient: native.OAuthClient{ClientID: "<app-id>", ClientSecret: "<app-secret>"},
			AccessToken: "<access-token>",
		}},
		"acmeCo/facebook/dashboard-login": {"123456789,987654321", Credentials{Title: native.CredentialsTitleOAuth}},
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

			if config.AccountIDs != expected.accountIDs || config.Credentials != expected.credentials {
				test.Errorf("account_ids = %q, credentials = %+v, want %q and %+v", config.AccountIDs, config.Credentials, expected.accountIDs, expected.credentials)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/facebook/all-options"]

	var config Config = spec.Endpoint.Connector.Config

	expectedInsights := []CustomInsight{
		{
			Name:                   "spend_by_age",
			Level:                  InsightLevelCampaign,
			Fields:                 "account_id,campaign_id,impressions,clicks,spend",
			Breakdowns:             "age,gender",
			ActionBreakdowns:       "action_type",
			StartDate:              "2024-06-01T00:00:00Z",
			InsightsLookbackWindow: 7,
		},
		{Name: "ad_spend", Fields: "account_id,ad_id,spend"},
	}

	expectedAdvanced := Advanced{IncludeDeleted: true, FetchThumbnailImages: true, FeatureFlags: "use_batch_api"}

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	if config.AccountIDs != "123456789,987654321" || config.StartDate != "2024-01-01T00:00:00Z" || config.InsightsLookbackWindow != 14 {
		test.Errorf("account_ids = %q, start_date = %q, insights_lookback_window = %d", config.AccountIDs, config.StartDate, config.InsightsLookbackWindow)
	}

	if !reflect.DeepEqual(config.CustomInsights, expectedInsights) {
		test.Errorf("custom_insights = %+v, want %+v", config.CustomInsights, expectedInsights)
	}

	if config.Advanced == nil || *config.Advanced != expectedAdvanced {
		test.Errorf("advanced = %+v, want %+v", config.Advanced, expectedAdvanced)
	}

	expectedBindings := []Binding{
		{Resource: Resource{Name: "campaigns", Interval: "PT1H"}, Target: "acmeCo/facebook/campaigns"},
		{Resource: Resource{Name: "spend_by_age", Interval: "PT1H"}, Target: "acmeCo/facebook/spend_by_age", Backfill: 1},
		{Resource: Resource{Name: "ad_creatives"}, Target: "acmeCo/facebook/ad_creatives", Disable: true},
	}

	if !reflect.DeepEqual(spec.Bindings, expectedBindings) {
		test.Errorf("bindings = %+v, want %+v", spec.Bindings, expectedBindings)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var accountIDs *regexp.Regexp = regexp.MustCompile(`^\d[\d,]+\d$`)
	var titles *spectest.EnumSamples[native.CredentialsTitle] = spectest.NewEnumSamples(test, "credentials_title", CredentialsTitles)
	var levels *spectest.EnumSamples[InsightLevel] = spectest.NewEnumSamples(test, "custom insight level", InsightLevels)
	var tokens map[bool]bool = map[bool]bool{}

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			if !accountIDs.MatchString(config.AccountIDs) {
				test.Errorf("%s: account_ids %q must be comma-separated numbers", name, config.AccountIDs)
			}

			titles.Add(name, config.Credentials.Title)

			tokens[config.Credentials.AccessToken != ""] = true

			for _, insight := range config.CustomInsights {
				if insight.Name == "" || insight.Fields == "" {
					test.Errorf("%s: custom insight %+v needs a name and fields", name, insight)
				}

				if insight.Level != "" {
					levels.Add(name, insight.Level)
				}
			}
		}
	}

	titles.AssertAllSampled()
	levels.AssertSampled(InsightLevelCampaign)

	if !tokens[true] || !tokens[false] {
		test.Errorf("captures sampled with an access_token = %v, want one with and one leaving it out", tokens)
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"refresh_token isn't part of Facebook's long-lived credentials": {
			Key: "refresh_token",
			Content: `
				captures:
					acmeCo/facebook/refresh-token:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-facebook-marketing-native:v1
								config:
									account_ids: "123456789"
									credentials:
										credentials_title: OAuth Credentials
										refresh_token: <refresh-token>
						bindings: []
			`,
		},
		"time_increment is a custom insight option of the legacy connector": {
			Key: "time_increment",
			Content: `
				captures:
					acmeCo/facebook/legacy-insight:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-facebook-marketing-native:v1
								config:
									account_ids: "123456789"
									custom_insights:
										- name: daily
											fields: spend
											breakdowns: ""
											time_increment: 1
									credentials:
										credentials_title: OAuth Credentials
						bindings: []
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}
