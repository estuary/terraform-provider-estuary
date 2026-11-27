package appleappstore

import (
	"reflect"
	"slices"
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

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]bool{
		"acmeCo/app-store/api-key":  true,
		"acmeCo/app-store/imported": false,
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

			var credentials Credentials = spec.Endpoint.Connector.Config.Credentials

			var want Credentials = Credentials{Title: native.CredentialsTitlePrivateApp}

			if keyed {
				want.PrivateKey, want.KeyID, want.IssuerID = credentials.PrivateKey, "<key-id>", "<issuer-id>"
			}

			if credentials != want || strings.HasPrefix(credentials.PrivateKey, "-----BEGIN PRIVATE KEY-----\n") != keyed {
				test.Errorf("credentials = %+v, want an API key with a multiline private key = %v", credentials, keyed)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/app-store/all-options"]

	var config Config = spec.Endpoint.Connector.Config

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	if !slices.Equal(config.AppIDs, []string{"1234567890", "9876543210"}) {
		test.Errorf("app_ids = %q, want both apps", config.AppIDs)
	}

	if credentials := config.Credentials; credentials.KeyID != "<key-id>" || credentials.IssuerID != "<issuer-id>" || credentials.AccessToken != "<access-token>" || !strings.Contains(credentials.PrivateKey, "<private-key>") {
		test.Errorf("credentials = %+v, want every field set", credentials)
	}

	expectedBindings := []Binding{
		{Resource: Resource{Name: "app_reviews", Interval: "PT1H"}, Target: "acmeCo/app-store/app_reviews"},
		{Resource: Resource{Name: "app_sessions", Interval: "PT6H"}, Target: "acmeCo/app-store/app_sessions", Backfill: 1},
		{Resource: Resource{Name: "app_crashes"}, Target: "acmeCo/app-store/app_crashes", Disable: true},
	}

	if !reflect.DeepEqual(spec.Bindings, expectedBindings) {
		test.Errorf("bindings = %+v, want %+v", spec.Bindings, expectedBindings)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var credentials Credentials = spec.Endpoint.Connector.Config.Credentials

			if credentials.Title != native.CredentialsTitlePrivateApp {
				test.Errorf("%s: credentials_title = %q, want the connector's only variant, %q", name, credentials.Title, native.CredentialsTitlePrivateApp)
			}

			if set := []bool{credentials.PrivateKey != "", credentials.KeyID != "", credentials.IssuerID != ""}; slices.Contains(set, true) && slices.Contains(set, false) {
				test.Errorf("%s: an API key needs private_key, key_id and issuer_id together", name)
			}
		}
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"start_date isn't an option of this connector": {
			Key: "start_date",
			Content: `
				captures:
					acmeCo/app-store/start-date:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-apple-app-store:v1
								config:
									start_date: "2024-01-01T00:00:00Z"
									credentials:
										credentials_title: Private App Credentials
						bindings: []
			`,
		},
		"vendor_number belongs to the sales reports API, which the connector doesn't use": {
			Key: "vendor_number",
			Content: `
				captures:
					acmeCo/app-store/vendor-number:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-apple-app-store:v1
								config:
									credentials:
										credentials_title: Private App Credentials
										vendor_number: "12345678"
						bindings: []
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}
