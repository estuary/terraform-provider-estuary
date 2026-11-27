package googlesearchconsole

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

// snapshotCredentials is Credentials as the shared conformance check reads the schema snapshot. The connector marks
// its secrets with airbyte_secret, which the check doesn't read, so it takes them for plain fields.
// TestSecretsMatchTheAirbyteSecretsOfTheSchema checks them against the snapshot instead.
type snapshotCredentials struct {
	Credentials        `yaml:",inline"`
	ClientID           string `yaml:"client_id,omitempty" secret:"false"`
	ClientSecret       string `yaml:"client_secret,omitempty" secret:"false"`
	RefreshToken       string `yaml:"refresh_token,omitempty" secret:"false"`
	ServiceAccountInfo string `yaml:"service_account_info,omitempty" secret:"false"`
}

// snapshotConfig is Config with its credentials as the conformance check reads them.
type snapshotConfig struct {
	Config      `yaml:",inline"`
	Credentials snapshotCredentials `yaml:"credentials"`
}

func TestSpec(test *testing.T) {
	spectest.RunCaptureSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[snapshotConfig, Resource](test)
}

// The credentials fields the spec keeps write-only are exactly the properties the schema marks airbyte_secret.
func TestSecretsMatchTheAirbyteSecretsOfTheSchema(test *testing.T) {
	var schema catalog.ConnectorSchema
	var marked, secret []string

	content, err := os.ReadFile(spectest.SchemaFile)

	if err == nil {
		err = json.Unmarshal(content, &schema)
	}

	if err != nil {
		test.Fatal(err)
	}

	variants, _ := schema.Endpoint["properties"].(map[string]any)["credentials"].(map[string]any)["oneOf"].([]any)

	for _, variant := range variants {
		properties, _ := variant.(map[string]any)["properties"].(map[string]any)

		for key, property := range properties {
			if flagged, _ := property.(map[string]any)["airbyte_secret"].(bool); flagged {
				marked = append(marked, key)
			}
		}
	}

	var credentials reflect.Type = reflect.TypeFor[Credentials]()

	for index := range credentials.NumField() {
		if field := credentials.Field(index); catalog.SecretField(field) {
			secret = append(secret, catalog.FieldKey(field))
		}
	}

	slices.Sort(marked)
	slices.Sort(secret)

	if len(marked) == 0 || !slices.Equal(marked, secret) {
		test.Errorf("write-only credentials = %q, the schema marks %q as airbyte_secret", secret, marked)
	}

	if fields := reflect.TypeFor[snapshotCredentials]().NumField(); fields != len(secret)+1 {
		test.Errorf("snapshotCredentials declares %d fields, want Credentials and its %d secrets only", fields, len(secret))
	}
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, AuthTypes, SyncModes)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		siteURL       string
		customReports bool
		credentials   Credentials
	}{
		"acmeCo/search-console/service-account": {"https://www.acme.example/", false, Credentials{AuthType: AuthTypeService, Email: "admin@acme.example"}},
		"acmeCo/search-console/oauth":           {"sc-domain:acme.example", false, Credentials{AuthType: AuthTypeClient, ClientID: "<client-id>", ClientSecret: "<client-secret>", RefreshToken: "<refresh-token>"}},
		"acmeCo/search-console/dashboard-login": {"sc-domain:acme.example", true, Credentials{AuthType: AuthTypeClient}},
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

			var credentials Credentials = config.Credentials

			if expected.credentials.AuthType == AuthTypeService {
				if !strings.Contains(credentials.ServiceAccountInfo, `"type": "service_account"`) {
					test.Errorf("service_account_info = %q, want a JSON key", credentials.ServiceAccountInfo)
				}

				credentials.ServiceAccountInfo = ""
			}

			if !slices.Equal(config.SiteURLs, []string{expected.siteURL}) || config.StartDate != "2024-01-01" || credentials != expected.credentials {
				test.Errorf("site_urls = %q, start_date = %q, credentials = %+v, want %q, 2024-01-01 and %+v", config.SiteURLs, config.StartDate, credentials, expected.siteURL, expected.credentials)
			}

			if (config.CustomReports != "") != expected.customReports {
				test.Errorf("custom_reports = %q, want set = %v", config.CustomReports, expected.customReports)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/search-console/all-options"]

	var config Config = spec.Endpoint.Connector.Config

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	if !slices.Equal(config.SiteURLs, []string{"https://www.acme.example/", "sc-domain:acme.example"}) || config.StartDate != "2024-01-01" || config.EndDate != "2024-12-31" {
		test.Errorf("site_urls = %q, start_date = %q, end_date = %q", config.SiteURLs, config.StartDate, config.EndDate)
	}

	var reports []struct {
		Name       string   `json:"name"`
		Dimensions []string `json:"dimensions"`
	}

	if err := json.Unmarshal([]byte(config.CustomReports), &reports); err != nil || len(reports) != 1 || reports[0].Name != "by_country_and_device" || len(reports[0].Dimensions) != 3 {
		test.Errorf("custom_reports = %q (%v), want a JSON array of one report with three dimensions", config.CustomReports, err)
	}

	expectedBindings := []Binding{
		{Resource: Resource{Stream: "search_analytics_by_date", SyncMode: SyncModeIncremental, CursorField: []string{"date"}}, Target: "acmeCo/search-console/search_analytics_by_date"},
		{Resource: Resource{Stream: "by_country_and_device", SyncMode: SyncModeIncremental, Namespace: "reports", CursorField: []string{"date"}}, Target: "acmeCo/search-console/by_country_and_device", Backfill: 1},
		{Resource: Resource{Stream: "sitemaps", SyncMode: SyncModeFullRefresh}, Target: "acmeCo/search-console/sitemaps", Disable: true},
	}

	if !reflect.DeepEqual(spec.Bindings, expectedBindings) {
		test.Errorf("bindings = %+v, want %+v", spec.Bindings, expectedBindings)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var authTypes *spectest.EnumSamples[AuthType] = spectest.NewEnumSamples(test, "auth_type", AuthTypes)
	var syncModes *spectest.EnumSamples[SyncMode] = spectest.NewEnumSamples(test, "syncMode", SyncModes)
	var customReports bool

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			var credentials Credentials = config.Credentials

			authTypes.Add(name, credentials.AuthType)

			customReports = customReports || config.CustomReports != ""

			if len(config.SiteURLs) == 0 || config.StartDate == "" {
				test.Errorf("%s: site_urls and start_date are required", name)
			}

			var oauth bool = credentials.ClientID != "" || credentials.ClientSecret != "" || credentials.RefreshToken != ""
			var service bool = credentials.ServiceAccountInfo != "" && credentials.Email != ""

			if (credentials.AuthType == AuthTypeService) != service || credentials.AuthType == AuthTypeService && oauth {
				test.Errorf("%s: %s credentials %+v must set the fields of their auth type and no other", name, credentials.AuthType, credentials)
			}

			for _, binding := range spec.Bindings {
				syncModes.Add(name, binding.Resource.SyncMode)
			}
		}
	}

	authTypes.AssertAllSampled()
	syncModes.AssertAllSampled()

	if !customReports {
		test.Error("no sample with custom_reports")
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"credentials_title is the selector of native connectors": {
			Key: "credentials_title",
			Content: `
				captures:
					acmeCo/search-console/native:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-google-search-console:v2
								config:
									site_urls:
										- sc-domain:acme.example
									start_date: "2024-01-01"
									credentials:
										credentials_title: OAuth Credentials
						bindings: []
			`,
		},
		"name is the resource key of native connectors": {
			Key: "name",
			Content: `
				captures:
					acmeCo/search-console/name:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-google-search-console:v2
								config:
									site_urls:
										- sc-domain:acme.example
									start_date: "2024-01-01"
									credentials:
										auth_type: Client
						bindings:
							- resource:
									name: sites
									syncMode: full_refresh
								target: acmeCo/search-console/sites
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}

func TestResourcePath(test *testing.T) {
	expectations := map[string]struct {
		resource Resource
		want     []string
	}{
		"a stream without a namespace": {Resource{Stream: "sites", SyncMode: SyncModeFullRefresh}, []string{"sites"}},
		"a stream in a namespace":      {Resource{Stream: "sites", Namespace: "reports"}, []string{"reports", "sites"}},
	}

	for name, expected := range expectations {
		if path := expected.resource.ResourcePath(); !slices.Equal(path, expected.want) {
			test.Errorf("%s: resource path = %q, want %q", name, path, expected.want)
		}
	}
}
