package zendesksupport

import (
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

// snapshotCredentials is Credentials as the connector schema declares it: access_token_expires_at isn't a schema
// secret, while the spec keeps it write-only, see Credentials.
type snapshotCredentials struct {
	Credentials          `yaml:",inline"`
	AccessTokenExpiresAt string `yaml:"access_token_expires_at,omitempty" secret:"false"`
}

// snapshotConfig is Config with its credentials as the connector schema declares them.
type snapshotConfig struct {
	Config      `yaml:",inline"`
	Credentials snapshotCredentials `yaml:"credentials"`
}

func TestSpec(test *testing.T) {
	spectest.RunCaptureSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[snapshotConfig, native.SnapshotResource[Resource]](test)
}

// The only difference from the schema snapshot is the one snapshotCredentials declares: every other field of the
// snapshot types is the spec's own.
func TestAccessTokenExpiresAtIsTheOnlySecretTheSchemaDoesNotMark(test *testing.T) {
	field, found := reflect.TypeFor[Credentials]().FieldByName("AccessTokenExpiresAt")

	if !found || !catalog.SecretField(field) {
		test.Errorf("access_token_expires_at is secret = %v, want it write-only, as the connector rewrites it", found && catalog.SecretField(field))
	}

	if fields := reflect.TypeFor[snapshotCredentials]().NumField(); fields != 2 {
		test.Errorf("snapshotCredentials declares %d fields, want Credentials and access_token_expires_at only", fields)
	}

	if fields := reflect.TypeFor[snapshotConfig]().NumField(); fields != 2 {
		test.Errorf("snapshotConfig declares %d fields, want Config and credentials only", fields)
	}
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, CredentialsTitles)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]Credentials{
		"acmeCo/zendesk/api-token": {Title: CredentialsTitleAPIToken, Username: "support-bot@acme.example", Password: "<api-token>"},
		"acmeCo/zendesk/oauth": {
			Title:                native.CredentialsTitleOAuth,
			OAuthClient:          native.OAuthClient{ClientID: "<client-id>", ClientSecret: "<client-secret>"},
			RefreshToken:         "<refresh-token>",
			AccessToken:          "<access-token>",
			AccessTokenExpiresAt: "2026-01-01T00:30:00Z",
		},
		"acmeCo/zendesk/dashboard-login": {Title: native.CredentialsTitleOAuth},
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

			if config.Subdomain != "acme" || config.Credentials != expected {
				test.Errorf("subdomain = %q, credentials = %+v, want acme and %+v", config.Subdomain, config.Credentials, expected)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/zendesk/all-options"]

	var config Config = spec.Endpoint.Connector.Config

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	if config.Subdomain != "acme-support" || config.StartDate != "2024-01-01T00:00:00Z" {
		test.Errorf("subdomain = %q, start_date = %q", config.Subdomain, config.StartDate)
	}

	expectations := []Binding{
		{Resource: Resource{Name: "tickets", Interval: "PT5M"}, Target: "acmeCo/zendesk/tickets"},
		{Resource: Resource{Name: "ticket_metrics", Interval: "PT1H"}, Target: "acmeCo/zendesk/ticket_metrics", Backfill: 1},
		{Resource: Resource{Name: "organizations"}, Target: "acmeCo/zendesk/organizations", Disable: true},
	}

	if !reflect.DeepEqual(spec.Bindings, expectations) {
		test.Errorf("bindings = %+v, want %+v", spec.Bindings, expectations)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var titles *spectest.EnumSamples[native.CredentialsTitle] = spectest.NewEnumSamples(test, "credentials_title", CredentialsTitles)

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var credentials Credentials = spec.Endpoint.Connector.Config.Credentials

			var oauth Credentials = credentials

			oauth.Title, oauth.Username, oauth.Password = "", "", ""

			titles.Add(name, credentials.Title)

			if credentials.Title == CredentialsTitleAPIToken && (credentials.Username == "" || credentials.Password == "" || oauth != Credentials{}) {
				test.Errorf("%s: Email & API Token credentials need username and password and no OAuth field", name)
			}

			if credentials.Title == native.CredentialsTitleOAuth && (credentials.Username != "" || credentials.Password != "") {
				test.Errorf("%s: OAuth Credentials take no username or password", name)
			}
		}
	}

	titles.AssertAllSampled()
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"schedule is a resource field of other native connectors": {
			Key: "schedule",
			Content: `
				captures:
					acmeCo/zendesk/schedule:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-zendesk-support-native:v1
								config:
									subdomain: acme
									credentials:
										credentials_title: Email & API Token
						bindings:
							- resource:
									name: tickets
									schedule: 0 0 * * *
								target: acmeCo/zendesk/tickets
			`,
		},
		"instance_url is a Salesforce credentials field": {
			Key: "instance_url",
			Content: `
				captures:
					acmeCo/zendesk/instance-url:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-zendesk-support-native:v1
								config:
									subdomain: acme
									credentials:
										credentials_title: OAuth Credentials
										instance_url: https://acme.zendesk.com
						bindings: []
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}
