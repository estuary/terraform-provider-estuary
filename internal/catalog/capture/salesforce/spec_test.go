package salesforce

import (
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
	spectest.AssertEnumTags[Spec](test, CredentialsTitles, WindowTypes)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		credentials Credentials
		myDomain    string
		windowSize  *WindowSize
		resource    Resource
	}{
		"acmeCo/salesforce/client-credentials": {
			credentials: Credentials{Title: CredentialsTitleClientCredentials, OAuthClient: native.OAuthClient{ClientID: "<consumer-key>", ClientSecret: "<consumer-secret>"}},
			myDomain:    "acme.my.salesforce.com",
			resource:    Resource{Resource: native.Resource{Name: "Account", Interval: "PT5M"}},
		},
		"acmeCo/salesforce/oauth": {
			credentials: Credentials{
				Title:        native.CredentialsTitleOAuth,
				OAuthClient:  native.OAuthClient{ClientID: "<client-id>", ClientSecret: "<client-secret>"},
				RefreshToken: "<refresh-token>",
				InstanceURL:  "https://acme.my.salesforce.com",
			},
			resource: Resource{Resource: native.Resource{Name: "Account", Interval: "PT5M"}, Schedule: "55 23 * * *"},
		},
		"acmeCo/salesforce/dashboard-login": {
			credentials: Credentials{Title: native.CredentialsTitleOAuth, InstanceURL: "https://acme.my.salesforce.com"},
			windowSize:  &WindowSize{WindowType: WindowTypeDays, Days: 30},
			resource:    Resource{Resource: native.Resource{Name: "Contact"}},
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

			var config Config = spec.Endpoint.Connector.Config

			if config.Credentials != expected.credentials || config.MyDomain != expected.myDomain {
				test.Errorf("credentials = %+v, my_domain = %q, want %+v and %q", config.Credentials, config.MyDomain, expected.credentials, expected.myDomain)
			}

			if (expected.windowSize == nil) != (config.Advanced == nil) || expected.windowSize != nil && *config.Advanced.WindowSize != *expected.windowSize {
				test.Errorf("advanced = %+v, want window_size %+v", config.Advanced, expected.windowSize)
			}

			if len(spec.Bindings) != 1 || spec.Bindings[0].Resource != expected.resource {
				test.Errorf("bindings = %+v, want one of resource %+v", spec.Bindings, expected.resource)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/salesforce/all-options"]

	var config Config = spec.Endpoint.Connector.Config

	expectedWindowSize := WindowSize{WindowType: WindowTypeInterval, Interval: "PT6H"}

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	if config.StartDate != "2024-01-01T00:00:00Z" || !config.IsSandbox || config.MyDomain != "acme--uat.sandbox.my.salesforce.com" {
		test.Errorf("start_date = %q, is_sandbox = %v, my_domain = %q", config.StartDate, config.IsSandbox, config.MyDomain)
	}

	if config.Advanced == nil || config.Advanced.WindowSize == nil || *config.Advanced.WindowSize != expectedWindowSize {
		test.Errorf("advanced = %+v, want window_size %+v", config.Advanced, expectedWindowSize)
	}
}

func TestSampleBindings(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var bindings []Binding = specs["acmeCo/salesforce/all-options"].Bindings

	expectations := []Binding{
		{Resource: Resource{Resource: native.Resource{Name: "Account", Interval: "PT5M"}, Schedule: "55 23 * * *"}, Target: "acmeCo/salesforce/Account"},
		{Resource: Resource{Resource: native.Resource{Name: "Opportunity", Interval: "PT15M"}}, Target: "acmeCo/salesforce/Opportunity", Backfill: 2},
		{Resource: Resource{Resource: native.Resource{Name: "Invoice__c", Interval: "PT1H"}, Schedule: "0 6 * * 1"}, Target: "acmeCo/salesforce/Invoice__c", Disable: true},
	}

	if len(bindings) != len(expectations) {
		test.Fatalf("got %d bindings, want %d", len(bindings), len(expectations))
	}

	for index, expected := range expectations {
		if bindings[index] != expected {
			test.Errorf("binding %d = %+v, want %+v", index, bindings[index], expected)
		}
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var titles *spectest.EnumSamples[native.CredentialsTitle] = spectest.NewEnumSamples(test, "credentials_title", CredentialsTitles)
	var windowTypes *spectest.EnumSamples[WindowType] = spectest.NewEnumSamples(test, "window_type", WindowTypes)
	var scheduled map[bool]bool = map[bool]bool{}

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			var credentials Credentials = config.Credentials

			titles.Add(name, credentials.Title)

			if credentials.Title == CredentialsTitleClientCredentials && (credentials.ClientID == "" || credentials.ClientSecret == "" || credentials.RefreshToken != "" || credentials.InstanceURL != "" || config.MyDomain == "") {
				test.Errorf("%s: Client Credentials need client_id, client_secret and my_domain, and no refresh_token or instance_url", name)
			}

			if credentials.Title == native.CredentialsTitleOAuth && credentials.InstanceURL == "" {
				test.Errorf("%s: OAuth Credentials need instance_url", name)
			}

			if config.Advanced != nil && config.Advanced.WindowSize != nil {
				var windowSize WindowSize = *config.Advanced.WindowSize

				windowTypes.Add(name, windowSize.WindowType)

				if (windowSize.WindowType == WindowTypeInterval) != (windowSize.Interval != "") || (windowSize.WindowType == WindowTypeDays) != (windowSize.Days > 0) {
					test.Errorf("%s: window_size %+v must set only the size of its window_type", name, windowSize)
				}
			}

			for _, binding := range spec.Bindings {
				scheduled[binding.Resource.Schedule != ""] = true
			}
		}
	}

	titles.AssertAllSampled()
	windowTypes.AssertAllSampled()

	if !scheduled[true] || !scheduled[false] {
		test.Errorf("bindings sampled with a schedule = %v, want one with and one without", scheduled)
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"username belongs to the deprecated credentials": {
			Key: "username",
			Content: `
				captures:
					acmeCo/salesforce/user-pass:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-salesforce-native:v1
								config:
									credentials:
										credentials_title: Username, Password, & Security Token
										username: integration@acme.example
						bindings: []
			`,
		},
		"stream is the resource key of the legacy connector": {
			Key: "stream",
			Content: `
				captures:
					acmeCo/salesforce/legacy:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-salesforce-native:v1
								config:
									credentials:
										credentials_title: Client Credentials
						bindings:
							- resource:
									stream: Account
								target: acmeCo/salesforce/Account
			`,
		},
		"_meta is the control plane's, not a resource field": {
			Key: "_meta",
			Content: `
				captures:
					acmeCo/salesforce/meta:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-salesforce-native:v1
								config:
									credentials:
										credentials_title: Client Credentials
						bindings:
							- resource:
									name: Account
									_meta:
										path: [Account]
								target: acmeCo/salesforce/Account
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}
