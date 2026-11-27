package stripe

import (
	"reflect"
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

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		credentials Credentials
		resource    Resource
	}{
		"acmeCo/stripe/api-key": {
			credentials: Credentials{Title: native.CredentialsTitlePrivateApp, AccessToken: "<api-key>"},
			resource:    Resource{Resource: native.Resource{Name: "Charges", Interval: "PT5M"}},
		},
		"acmeCo/stripe/imported": {
			credentials: Credentials{Title: native.CredentialsTitlePrivateApp},
			resource:    Resource{Resource: native.Resource{Name: "Accounts", Interval: "PT5M"}, Schedule: "0 0 * * *"},
		},
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

			if credentials := spec.Endpoint.Connector.Config.Credentials; credentials != expected.credentials {
				test.Errorf("credentials = %+v, want %+v", credentials, expected.credentials)
			}

			if len(spec.Bindings) != 1 || spec.Bindings[0].Resource != expected.resource {
				test.Errorf("bindings = %+v, want one of resource %+v", spec.Bindings, expected.resource)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/stripe/all-options"]

	var config Config = spec.Endpoint.Connector.Config

	expectedAdvanced := Advanced{APIVersion: "2024-06-20", IncrementalWindowSize: "PT1H"}

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	if config.StartDate != "2024-01-01T00:00:00Z" || !config.CaptureConnectedAccounts {
		test.Errorf("start_date = %q, capture_connected_accounts = %v", config.StartDate, config.CaptureConnectedAccounts)
	}

	if config.Advanced == nil || *config.Advanced != expectedAdvanced {
		test.Errorf("advanced = %+v, want %+v", config.Advanced, expectedAdvanced)
	}

	expectations := []Binding{
		{Resource: Resource{Resource: native.Resource{Name: "Charges", Interval: "PT5M"}}, Target: "acmeCo/stripe/Charges"},
		{Resource: Resource{Resource: native.Resource{Name: "Accounts", Interval: "PT5M"}, Schedule: "0 0 * * *"}, Target: "acmeCo/stripe/Accounts", Backfill: 3},
		{Resource: Resource{Resource: native.Resource{Name: "Customers"}}, Target: "acmeCo/stripe/Customers", Disable: true},
	}

	if !reflect.DeepEqual(spec.Bindings, expectations) {
		test.Errorf("bindings = %+v, want %+v", spec.Bindings, expectations)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var keyed map[bool]bool = map[bool]bool{}

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var credentials Credentials = spec.Endpoint.Connector.Config.Credentials

			if credentials.Title != native.CredentialsTitlePrivateApp {
				test.Errorf("%s: credentials_title = %q, want the connector's only variant, %q", name, credentials.Title, native.CredentialsTitlePrivateApp)
			}

			keyed[credentials.AccessToken != ""] = true
		}
	}

	if !keyed[true] || !keyed[false] {
		test.Errorf("captures sampled with an access_token = %v, want one with and one leaving it out", keyed)
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"client_id belongs to OAuth credentials, which Stripe doesn't have": {
			Key: "client_id",
			Content: `
				captures:
					acmeCo/stripe/oauth:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-stripe-native:v1
								config:
									credentials:
										credentials_title: Private App Credentials
										client_id: <client-id>
						bindings: []
			`,
		},
		"syncMode is a resource key of the legacy connector": {
			Key: "syncMode",
			Content: `
				captures:
					acmeCo/stripe/legacy:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-stripe-native:v1
								config:
									credentials:
										credentials_title: Private App Credentials
						bindings:
							- resource:
									name: Charges
									syncMode: incremental
								target: acmeCo/stripe/Charges
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}
