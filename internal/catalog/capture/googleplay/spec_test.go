package googleplay

import (
	"reflect"
	"strings"
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
	spectest.AssertEnumTags[Spec](test, CredentialsTitles)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]bool{
		"acmeCo/google-play/service-account": true,
		"acmeCo/google-play/imported":        false,
	}

	if len(specs) != len(expectations) {
		test.Fatalf("got %d captures, want %d", len(specs), len(expectations))
	}

	for name, keyed := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			if !found {
				test.Fatal("capture missing from sample")
			}

			var config Config = spec.Endpoint.Connector.Config

			if config.Bucket != "pubsite_prod_1234567890123456789" || config.Credentials.Title != CredentialsTitleServiceAccount {
				test.Errorf("bucket = %q, credentials_title = %q", config.Bucket, config.Credentials.Title)
			}

			if strings.Contains(config.Credentials.ServiceAccount, `"type": "service_account"`) != keyed {
				test.Errorf("service_account = %q, want a JSON key = %v", config.Credentials.ServiceAccount, keyed)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/google-play/all-options"]

	var config Config = spec.Endpoint.Connector.Config

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	if config.Bucket != "pubsite_prod_1234567890123456789" || config.StartDate != "2024-01-01T00:00:00Z" {
		test.Errorf("bucket = %q, start_date = %q", config.Bucket, config.StartDate)
	}

	expectedBindings := []Binding{
		{Resource: Resource{Name: "installs", Interval: "PT1H"}, Target: "acmeCo/google-play/installs"},
		{Resource: Resource{Name: "crashes", Interval: "PT6H"}, Target: "acmeCo/google-play/crashes", Backfill: 1},
		{Resource: Resource{Name: "reviews"}, Target: "acmeCo/google-play/reviews", Disable: true},
	}

	if !reflect.DeepEqual(spec.Bindings, expectedBindings) {
		test.Errorf("bindings = %+v, want %+v", spec.Bindings, expectedBindings)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var titles *spectest.EnumSamples[native.CredentialsTitle] = spectest.NewEnumSamples(test, "credentials_title", CredentialsTitles)

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			titles.Add(name, config.Credentials.Title)

			if !strings.HasPrefix(config.Bucket, "pubsite_prod") {
				test.Errorf("%s: bucket %q must start with pubsite_prod", name, config.Bucket)
			}
		}
	}

	titles.AssertAllSampled()
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"credentials_json is the key of other Google connectors": {
			Key: "credentials_json",
			Content: `
				captures:
					acmeCo/google-play/credentials-json:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-google-play:v1
								config:
									bucket: pubsite_prod_1234567890123456789
									credentials:
										credentials_title: Google Service Account
										credentials_json: "{}"
						bindings: []
			`,
		},
		"schedule is a resource field of other native connectors": {
			Key: "schedule",
			Content: `
				captures:
					acmeCo/google-play/schedule:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-google-play:v1
								config:
									bucket: pubsite_prod_1234567890123456789
									credentials:
										credentials_title: Google Service Account
						bindings:
							- resource:
									name: installs
									schedule: 0 0 * * *
								target: acmeCo/google-play/installs
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}
