package builder

import (
	"context"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/zendesksupport"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var zendeskSupportConnector Connector = Connector{
	TypeName:       "source_zendesk_support",
	Image:          zendesksupport.Image,
	ConnectorName:  zendesksupport.ConnectorName,
	RotatesSecrets: true,
}

// zendeskSupportLive is a Zendesk Support capture created through the dashboard login: the sealed credentials of
// Estuary's OAuth client, the expiry of its access token, and discovered bindings with their resource paths.
const zendeskSupportLive string = `
	autoDiscover:
		addNewBindings: true
		evolveIncompatibleCollections: true
	endpoint:
		connector:
			image: ghcr.io/estuary/source-zendesk-support-native:v1
			config:
				subdomain: acme
				start_date: "2024-01-01T00:00:00Z"
				credentials:
					credentials_title: OAuth Credentials
					client_id_sops: ENC[AES256_GCM,data:abc,type:str]
					client_secret_sops: ENC[AES256_GCM,data:def,type:str]
					refresh_token_sops: ENC[AES256_GCM,data:ghi,type:str]
					access_token_sops: ENC[AES256_GCM,data:jkl,type:str]
					access_token_expires_at: "2026-01-01T00:30:00Z"
				sops:
					gcp_kms:
						- resource_id: projects/example/locations/us-central1/keyRings/example/cryptoKeys/example
							created_at: "2026-01-01T00:00:00Z"
							enc: <encrypted>
					lastmodified: "2026-01-01T00:00:00Z"
					mac: ENC[AES256_GCM,data:mno,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				_meta:
					path: [tickets]
				name: tickets
				interval: PT5M
			target: acmeCo/zendesk/tickets
			backfill: 1
		- resource:
				_meta:
					path: [users]
				name: users
				interval: PT5M
			target: acmeCo/zendesk/users
	shards:
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

// zendeskSupportAPITokenLive is a Zendesk Support capture authenticating with an email and an API token.
const zendeskSupportAPITokenLive string = `
	endpoint:
		connector:
			image: ghcr.io/estuary/source-zendesk-support-native:v1
			config:
				subdomain: acme
				credentials:
					credentials_title: Email & API Token
					username: support-bot@acme.example
					password_sops: ENC[AES256_GCM,data:abc,type:str]
				sops:
					mac: ENC[AES256_GCM,data:mno,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				_meta:
					path: [tickets]
				name: tickets
				interval: PT5M
			target: acmeCo/zendesk/tickets
`

func zendeskSupportSpec(discovers bool, config zendesksupport.Config, bindings ...capture.Binding[zendesksupport.Resource]) CaptureSpec[zendesksupport.Config, zendesksupport.Resource] {
	var spec CaptureSpec[zendesksupport.Config, zendesksupport.Resource] = CaptureSpec[zendesksupport.Config, zendesksupport.Resource]{
		Endpoint: endpoint.ConnectorSpec[zendesksupport.Config]{Connector: endpoint.Connector[zendesksupport.Config]{Image: zendesksupport.Image, Config: config}},
		Bindings: bindings,
	}

	if discovers {
		spec.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true, EvolveIncompatibleCollections: true}
	}

	return spec
}

func zendeskSupportBinding(name string) capture.Binding[zendesksupport.Resource] {
	return capture.Binding[zendesksupport.Resource]{
		Resource: zendesksupport.Resource{Name: name, Interval: "PT5M"},
		Target:   "acmeCo/zendesk/" + name,
	}
}

// rotateZendeskSupportTokens rewrites the live spec as the connector does when it refreshes its access token: new
// encrypted tokens, a new expiry, new sops metadata and a new publication.
func rotateZendeskSupportTokens(live map[string]any) {
	var config map[string]any = utils.Lookup(live, catalog.EndpointKey, catalog.ConnectorKey, catalog.ConfigKey)
	var credentials map[string]any = utils.Lookup(config, "credentials")
	var sops map[string]any = utils.Lookup(config, "sops")

	credentials["refresh_token_sops"] = "ENC[AES256_GCM,data:rotated-refresh,type:str]"
	credentials["access_token_sops"] = "ENC[AES256_GCM,data:rotated-access,type:str]"
	credentials["access_token_expires_at"] = "2026-01-01T01:00:00Z"
	sops["lastmodified"] = "2026-01-01T00:30:00Z"
	sops["mac"] = "ENC[AES256_GCM,data:rotated-mac,type:str]"

	live[catalog.ExpectPubIDKey] = "11:22:33:44:55:66:77:99"
}

func TestZendeskSupportSamplesRoundTrip(test *testing.T) {
	assertCaptureSamplesRoundTrip[zendesksupport.Config, zendesksupport.Resource](test, zendeskSupportConnector, "../../catalog/capture/zendesksupport/testdata")
}

func TestZendeskSupportImportPlansNoChanges(test *testing.T) {
	var config zendesksupport.Config = zendesksupport.Config{
		Subdomain:   "acme",
		Credentials: zendesksupport.Credentials{Title: zendesksupport.CredentialsTitleAPIToken, Username: "support-bot@acme.example"},
	}

	entry := newConnectorCapture[zendesksupport.Config, zendesksupport.Resource](test, zendeskSupportConnector, zendeskSupportAPITokenLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/support/source-zendesk-support-native", false)

	entry.assertPlansNoChanges(test, "acmeCo/support", zendeskSupportSpec(false, config), state)
	entry.assertAppliesWithoutPublishing(test, "acmeCo/support", zendeskSupportSpec(false, config, zendeskSupportBinding("tickets")), state)
}

// A capture created through the dashboard login imports with its OAuth secrets left out of the configuration, and
// keeps planning no changes after the connector rotates its tokens.
func TestZendeskSupportImportOfADashboardLoginPlansNoChangesAcrossRotations(test *testing.T) {
	var ctx context.Context = context.Background()
	var config zendesksupport.Config = zendesksupport.Config{
		Subdomain:   "acme",
		StartDate:   "2024-01-01T00:00:00Z",
		Credentials: zendesksupport.Credentials{Title: native.CredentialsTitleOAuth},
	}
	var declared CaptureSpec[zendesksupport.Config, zendesksupport.Resource] = zendeskSupportSpec(true, config, zendeskSupportBinding("tickets"), zendeskSupportBinding("users"))

	entry := newConnectorCapture[zendesksupport.Config, zendesksupport.Resource](test, zendeskSupportConnector, zendeskSupportLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/support/source-zendesk-support-native", false)

	entry.assertPlansNoChanges(test, "acmeCo/support", zendeskSupportSpec(true, config), state)
	entry.assertAppliesWithoutPublishing(test, "acmeCo/support", declared, state)

	rotateZendeskSupportTokens(entry.backend.live)

	var readResp resource.ReadResponse = resource.ReadResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: state}}

	entry.Read(ctx, resource.ReadRequest{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: state}}, &readResp)

	if readResp.Diagnostics.HasError() || readResp.Diagnostics.WarningsCount() > 0 {
		test.Fatalf("refresh after the rotation: %v, want no diagnostics", readResp.Diagnostics)
	}

	if !readResp.State.Raw.Equal(state) {
		test.Errorf("refreshed state differs after the rotation, want the rotated tokens and their expiry out of state:\n%s", valueDifferences(readResp.State.Raw, state))
	}

	entry.assertPlansNoChanges(test, "acmeCo/support", zendeskSupportSpec(true, config), readResp.State.Raw)
	entry.assertAppliesWithoutPublishing(test, "acmeCo/support", declared, readResp.State.Raw)
}

// Tokens the configuration sets, from the user's own OAuth client, aren't reported as changed outside Terraform once
// the connector rotates them, which only RotatesSecrets prevents.
func TestZendeskSupportRotatedTokensAreNotDrift(test *testing.T) {
	var spec CaptureSpec[zendesksupport.Config, zendesksupport.Resource] = zendeskSupportSpec(false, zendesksupport.Config{
		Subdomain: "acme",
		StartDate: "2024-01-01T00:00:00Z",
		Credentials: zendesksupport.Credentials{
			Title:                native.CredentialsTitleOAuth,
			OAuthClient:          native.OAuthClient{ClientID: "<client-id>", ClientSecret: "<client-secret>"},
			RefreshToken:         "<refresh-token>",
			AccessToken:          "<access-token>",
			AccessTokenExpiresAt: "2026-01-01T00:30:00Z",
		},
	})

	for name, rotates := range map[string]bool{"a connector rotating its secrets": true, "a connector that doesn't": false} {
		var connector Connector = zendeskSupportConnector

		connector.RotatesSecrets = rotates

		entry := newConnectorCapture[zendesksupport.Config, zendesksupport.Resource](test, connector, zendeskSupportLive)

		config, err := entry.codec.Value("support", spec)

		if err != nil {
			test.Fatal(err)
		}

		plan, err := entry.codec.secrets.nullSecrets(config)

		if err != nil {
			test.Fatal(err)
		}

		state, err := entry.codec.secrets.stateHashes(plan, config)

		if err != nil {
			test.Fatal(err)
		}

		var published string = catalog.SecretsFingerprint(entry.backend.live)

		rotateZendeskSupportTokens(entry.backend.live)

		fingerprint, refreshed, diagnostics := entry.refreshedFingerprint("acmeCo/support/source-zendesk-support-native", published, entry.backend.live, state)

		var drifted bool = diagnostics.WarningsCount() > 0 || !refreshed.Equal(state)

		if diagnostics.HasError() || drifted == rotates {
			test.Errorf("%s: drift reported = %v (%v), want %v", name, drifted, diagnostics, !rotates)
		}

		if rotates && (fingerprint == published || fingerprint != catalog.SecretsFingerprint(entry.backend.live)) {
			test.Errorf("%s: kept fingerprint = %q, want the one of the rotated secrets", name, fingerprint)
		}
	}
}

func TestZendeskSupportSecretsAreWriteOnly(test *testing.T) {
	var spec CaptureSpec[zendesksupport.Config, zendesksupport.Resource] = zendeskSupportSpec(false, zendesksupport.Config{
		Subdomain: "acme",
		Credentials: zendesksupport.Credentials{
			Username:             "support-bot@acme.example",
			Password:             "<api-token>",
			OAuthClient:          native.OAuthClient{ClientID: "<client-id>", ClientSecret: "<client-secret>"},
			RefreshToken:         "<refresh-token>",
			AccessToken:          "<access-token>",
			AccessTokenExpiresAt: "2026-01-01T00:30:00Z",
		},
	})

	codec, err := newCodec[captureDocument[zendesksupport.Config, zendesksupport.Resource]](zendeskSupportConnector, captureType[zendesksupport.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	config, err := codec.Value("support", spec)

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, codec.secrets, config,
		configAttributePath("credentials", "password"),
		configAttributePath("credentials", "client_id"),
		configAttributePath("credentials", "client_secret"),
		configAttributePath("credentials", "refresh_token"),
		configAttributePath("credentials", "access_token"),
		configAttributePath("credentials", "access_token_expires_at"),
	)

	for _, name := range []string{"credentials_title", "username"} {
		if path := configAttributePath("credentials", name); codec.secrets.attribute(path) {
			test.Errorf("%s is a secret, want it readable in state", path)
		}
	}
}
