package builder

import (
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/salesforce"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var salesforceConnector Connector = Connector{
	TypeName:      "source_salesforce",
	Image:         salesforce.Image,
	ConnectorName: salesforce.ConnectorName,
}

// salesforceLive is a Salesforce capture created through the dashboard login: the sealed credentials of Estuary's
// OAuth app, explicit defaults, discovered bindings with their resource paths, and a runtime shard flag.
const salesforceLive string = `
	autoDiscover:
		addNewBindings: true
		evolveIncompatibleCollections: true
	endpoint:
		connector:
			image: ghcr.io/estuary/source-salesforce-native:v1
			config:
				credentials:
					credentials_title: OAuth Credentials
					client_id_sops: ENC[AES256_GCM,data:abc,type:str]
					client_secret_sops: ENC[AES256_GCM,data:def,type:str]
					refresh_token_sops: ENC[AES256_GCM,data:ghi,type:str]
					instance_url: https://acme.my.salesforce.com
				start_date: "2024-01-01T00:00:00Z"
				is_sandbox: false
				my_domain: ""
				advanced:
					window_size:
						window_type: days
						days: 18250
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
					path: [Account]
				name: Account
				interval: PT5M
				schedule: 55 23 * * *
			target: acmeCo/salesforce/Account
			backfill: 1
		- resource:
				_meta:
					path: [Contact]
				name: Contact
				interval: PT5M
			target: acmeCo/salesforce/Contact
		- resource:
				_meta:
					path: [Lead]
				name: Lead
			target: acmeCo/salesforce/Lead
			disable: true
	shards:
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

func salesforceSpec(config salesforce.Config, bindings ...capture.Binding[salesforce.Resource]) CaptureSpec[salesforce.Config, salesforce.Resource] {
	return CaptureSpec[salesforce.Config, salesforce.Resource]{
		AutoDiscover: &capture.AutoDiscover{AddNewBindings: true, EvolveIncompatibleCollections: true},
		Endpoint:     endpoint.ConnectorSpec[salesforce.Config]{Connector: endpoint.Connector[salesforce.Config]{Image: salesforce.Image, Config: config}},
		Bindings:     bindings,
	}
}

func salesforceBinding(name string, interval string, schedule string) capture.Binding[salesforce.Resource] {
	return capture.Binding[salesforce.Resource]{
		Resource: salesforce.Resource{Resource: native.Resource{Name: name, Interval: interval}, Schedule: schedule},
		Target:   "acmeCo/salesforce/" + name,
	}
}

func TestSalesforceSamplesRoundTrip(test *testing.T) {
	assertCaptureSamplesRoundTrip[salesforce.Config, salesforce.Resource](test, salesforceConnector, "../../catalog/capture/salesforce/testdata")
}

// A capture created through the dashboard login imports with its OAuth secrets left out of the configuration, which
// then plans no changes and applies without publishing.
func TestSalesforceImportOfADashboardLoginPlansNoChanges(test *testing.T) {
	var config salesforce.Config = salesforce.Config{
		Credentials: salesforce.Credentials{Title: native.CredentialsTitleOAuth, InstanceURL: "https://acme.my.salesforce.com"},
		StartDate:   "2024-01-01T00:00:00Z",
		Advanced:    &salesforce.Advanced{WindowSize: &salesforce.WindowSize{WindowType: salesforce.WindowTypeDays, Days: 18250}},
	}

	entry := newConnectorCapture[salesforce.Config, salesforce.Resource](test, salesforceConnector, salesforceLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/crm/source-salesforce-native", false)

	entry.assertPlansNoChanges(test, "acmeCo/crm", salesforceSpec(config), state)

	var disabled capture.Binding[salesforce.Resource] = salesforceBinding("Lead", "", "")

	disabled.Disable = true

	entry.assertAppliesWithoutPublishing(test, "acmeCo/crm", salesforceSpec(config,
		salesforceBinding("Account", "PT5M", "55 23 * * *"),
		salesforceBinding("Contact", "PT5M", ""),
		disabled,
	), state)
}

// Changing a binding Terraform takes over keeps the resource path the control plane recorded in its _meta, and the
// sealed endpoint, as the configuration sets no secret.
func TestSalesforceApplyKeepsTheResourceMetaOfAChangedBinding(test *testing.T) {
	var config salesforce.Config = salesforce.Config{
		Credentials: salesforce.Credentials{Title: native.CredentialsTitleOAuth, InstanceURL: "https://acme.my.salesforce.com"},
		StartDate:   "2024-01-01T00:00:00Z",
		Advanced:    &salesforce.Advanced{WindowSize: &salesforce.WindowSize{WindowType: salesforce.WindowTypeDays, Days: 18250}},
	}

	entry := newConnectorCapture[salesforce.Config, salesforce.Resource](test, salesforceConnector, salesforceLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/crm/source-salesforce-native", false)

	published, _ := entry.apply(test, "acmeCo/crm", salesforceSpec(config, salesforceBinding("Account", "PT15M", "0 6 * * *")), state)

	var want []any = catalog.Bindings(entry.backend.live)

	want[0] = map[string]any{
		"resource": map[string]any{"_meta": map[string]any{"path": []any{"Account"}}, "name": "Account", "interval": "PT15M", "schedule": "0 6 * * *"},
		"target":   "acmeCo/salesforce/Account",
		"backfill": 1,
	}

	if !reflect.DeepEqual(catalog.Bindings(published), want) {
		test.Errorf("published bindings = %v, want Account changed with its _meta and backfill counter kept, and the others as they are live", catalog.Bindings(published))
	}

	if !reflect.DeepEqual(published[catalog.EndpointKey], entry.backend.live[catalog.EndpointKey]) {
		test.Errorf("published endpoint = %v, want the sealed live endpoint", published[catalog.EndpointKey])
	}
}

func TestSalesforceSecretsAreWriteOnly(test *testing.T) {
	var spec CaptureSpec[salesforce.Config, salesforce.Resource] = salesforceSpec(salesforce.Config{
		Credentials: salesforce.Credentials{
			Title:        native.CredentialsTitleOAuth,
			OAuthClient:  native.OAuthClient{ClientID: "<client-id>", ClientSecret: "<client-secret>"},
			RefreshToken: "<refresh-token>",
			InstanceURL:  "https://acme.my.salesforce.com",
		},
	})

	codec, err := newCodec[captureDocument[salesforce.Config, salesforce.Resource]](salesforceConnector, captureType[salesforce.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	config, err := codec.Value("crm", spec)

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, codec.secrets, config,
		configAttributePath("credentials", "client_id"),
		configAttributePath("credentials", "client_secret"),
		configAttributePath("credentials", "refresh_token"),
	)

	for _, name := range []string{"credentials_title", "instance_url"} {
		if path := configAttributePath("credentials", name); codec.secrets.attribute(path) {
			test.Errorf("%s is a secret, want it readable in state", path)
		}
	}
}
