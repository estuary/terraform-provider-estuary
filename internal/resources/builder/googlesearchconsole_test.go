package builder

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/googlesearchconsole"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var googleSearchConsoleConnector Connector = Connector{
	TypeName:      "source_google_search_console",
	Image:         googlesearchconsole.Image,
	ConnectorName: googlesearchconsole.ConnectorName,
}

// googleSearchConsoleBindings are the bindings of a Google Search Console capture as discovery publishes them: an
// incremental stream with its cursor, a custom report and a full refresh stream, with their resource paths.
const googleSearchConsoleBindings string = `
	bindings:
		- resource:
				_meta:
					path: [search_analytics_by_date]
				stream: search_analytics_by_date
				syncMode: incremental
				cursorField: [date]
			target: acmeCo/search-console/search_analytics_by_date
			backfill: 1
		- resource:
				_meta:
					path: [by_country_and_device]
				stream: by_country_and_device
				syncMode: incremental
				cursorField: [date]
			target: acmeCo/search-console/by_country_and_device
		- resource:
				_meta:
					path: [sites]
				stream: sites
				syncMode: full_refresh
			target: acmeCo/search-console/sites
	shards:
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

// googleSearchConsoleLive is a Google Search Console capture created through the dashboard login: the sealed
// credentials of Estuary's OAuth client, and a custom report.
const googleSearchConsoleLive string = `
	autoDiscover:
		addNewBindings: true
		evolveIncompatibleCollections: true
	endpoint:
		connector:
			image: ghcr.io/estuary/source-google-search-console:v2
			config:
				site_urls:
					- sc-domain:acme.example
				start_date: "2024-01-01"
				custom_reports: '[{"name": "by_country_and_device", "dimensions": ["date", "country", "device"]}]'
				credentials:
					auth_type: Client
					client_id_sops: ENC[AES256_GCM,data:abc,type:str]
					client_secret_sops: ENC[AES256_GCM,data:def,type:str]
					refresh_token_sops: ENC[AES256_GCM,data:ghi,type:str]
				sops:
					gcp_kms:
						- resource_id: projects/example/locations/us-central1/keyRings/example/cryptoKeys/example
							created_at: "2026-01-01T00:00:00Z"
							enc: <encrypted>
					lastmodified: "2026-01-01T00:00:00Z"
					mac: ENC[AES256_GCM,data:jkl,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
` + googleSearchConsoleBindings

// googleSearchConsoleServiceLive is a Google Search Console capture authenticating with a service account key.
const googleSearchConsoleServiceLive string = `
	endpoint:
		connector:
			image: ghcr.io/estuary/source-google-search-console:v2
			config:
				site_urls:
					- https://www.acme.example/
					- sc-domain:acme.example
				start_date: "2024-01-01"
				end_date: "2024-12-31"
				credentials:
					auth_type: Service
					service_account_info_sops: ENC[AES256_GCM,data:abc,type:str]
					email: admin@acme.example
				sops:
					mac: ENC[AES256_GCM,data:jkl,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
` + googleSearchConsoleBindings

// googleSearchConsoleDeclared declares the bindings of googleSearchConsoleBindings.
var googleSearchConsoleDeclared []capture.Binding[googlesearchconsole.Resource] = []capture.Binding[googlesearchconsole.Resource]{
	{
		Resource: googlesearchconsole.Resource{Stream: "search_analytics_by_date", SyncMode: googlesearchconsole.SyncModeIncremental, CursorField: []string{"date"}},
		Target:   "acmeCo/search-console/search_analytics_by_date",
	},
	{
		Resource: googlesearchconsole.Resource{Stream: "by_country_and_device", SyncMode: googlesearchconsole.SyncModeIncremental, CursorField: []string{"date"}},
		Target:   "acmeCo/search-console/by_country_and_device",
	},
	{
		Resource: googlesearchconsole.Resource{Stream: "sites", SyncMode: googlesearchconsole.SyncModeFullRefresh},
		Target:   "acmeCo/search-console/sites",
	},
}

func googleSearchConsoleSpec(discovers bool, config googlesearchconsole.Config, bindings ...capture.Binding[googlesearchconsole.Resource]) CaptureSpec[googlesearchconsole.Config, googlesearchconsole.Resource] {
	var spec CaptureSpec[googlesearchconsole.Config, googlesearchconsole.Resource] = CaptureSpec[googlesearchconsole.Config, googlesearchconsole.Resource]{
		Endpoint: endpoint.ConnectorSpec[googlesearchconsole.Config]{Connector: endpoint.Connector[googlesearchconsole.Config]{Image: googlesearchconsole.Image, Config: config}},
		Bindings: bindings,
	}

	if discovers {
		spec.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true, EvolveIncompatibleCollections: true}
	}

	return spec
}

func TestGoogleSearchConsoleSamplesRoundTrip(test *testing.T) {
	assertCaptureSamplesRoundTrip[googlesearchconsole.Config, googlesearchconsole.Resource](test, googleSearchConsoleConnector, "../../catalog/capture/googlesearchconsole/testdata")
}

// A capture created through the dashboard login imports with its OAuth secrets left out of the configuration, which
// then plans no changes and applies without publishing.
func TestGoogleSearchConsoleImportOfADashboardLoginPlansNoChanges(test *testing.T) {
	var config googlesearchconsole.Config = googlesearchconsole.Config{
		SiteURLs:      []string{"sc-domain:acme.example"},
		StartDate:     "2024-01-01",
		CustomReports: `[{"name": "by_country_and_device", "dimensions": ["date", "country", "device"]}]`,
		Credentials:   googlesearchconsole.Credentials{AuthType: googlesearchconsole.AuthTypeClient},
	}

	entry := newConnectorCapture[googlesearchconsole.Config, googlesearchconsole.Resource](test, googleSearchConsoleConnector, googleSearchConsoleLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/seo/source-google-search-console", false)

	entry.assertPlansNoChanges(test, "acmeCo/seo", googleSearchConsoleSpec(true, config), state)
	entry.assertAppliesWithoutPublishing(test, "acmeCo/seo", googleSearchConsoleSpec(true, config, googleSearchConsoleDeclared...), state)
}

func TestGoogleSearchConsoleImportWithAServiceAccountPlansNoChanges(test *testing.T) {
	var config googlesearchconsole.Config = googlesearchconsole.Config{
		SiteURLs:    []string{"https://www.acme.example/", "sc-domain:acme.example"},
		StartDate:   "2024-01-01",
		EndDate:     "2024-12-31",
		Credentials: googlesearchconsole.Credentials{AuthType: googlesearchconsole.AuthTypeService, Email: "admin@acme.example"},
	}

	entry := newConnectorCapture[googlesearchconsole.Config, googlesearchconsole.Resource](test, googleSearchConsoleConnector, googleSearchConsoleServiceLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/seo/source-google-search-console", false)

	entry.assertPlansNoChanges(test, "acmeCo/seo", googleSearchConsoleSpec(false, config), state)
	entry.assertAppliesWithoutPublishing(test, "acmeCo/seo", googleSearchConsoleSpec(false, config, googleSearchConsoleDeclared...), state)
}

func TestGoogleSearchConsoleSecretsAreWriteOnly(test *testing.T) {
	codec, err := newCodec[captureDocument[googlesearchconsole.Config, googlesearchconsole.Resource]](googleSearchConsoleConnector, captureType[googlesearchconsole.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	config, err := codec.Value("seo", googleSearchConsoleSpec(false, googlesearchconsole.Config{
		SiteURLs:  []string{"sc-domain:acme.example"},
		StartDate: "2024-01-01",
		Credentials: googlesearchconsole.Credentials{
			AuthType:           googlesearchconsole.AuthTypeClient,
			ClientID:           "<client-id>",
			ClientSecret:       "<client-secret>",
			RefreshToken:       "<refresh-token>",
			ServiceAccountInfo: `{"type": "service_account"}`,
			Email:              "admin@acme.example",
		},
	}))

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, codec.secrets, config,
		configAttributePath("credentials", "client_id"),
		configAttributePath("credentials", "client_secret"),
		configAttributePath("credentials", "refresh_token"),
		configAttributePath("credentials", "service_account_info"),
	)

	for _, name := range []string{"auth_type", "email"} {
		if path := configAttributePath("credentials", name); codec.secrets.attribute(path) {
			test.Errorf("%s is a secret, want it readable in state", path)
		}
	}
}
