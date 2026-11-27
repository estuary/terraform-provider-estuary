package builder

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/mongodb"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var documentDBConnector Connector = Connector{
	TypeName:      "source_amazon_documentdb",
	Image:         mongodb.ImageAmazonDocumentDB,
	ConnectorName: mongodb.ConnectorNameAmazonDocumentDB,
}

// documentDBLive is an Amazon DocumentDB capture as the dashboard publishes it: a sealed password, explicit
// defaults, and batch bindings with their resource paths.
const documentDBLive string = `
	autoDiscover:
		addNewBindings: true
		evolveIncompatibleCollections: true
	endpoint:
		connector:
			image: ghcr.io/estuary/source-amazon-documentdb:v3
			config:
				address: mongodb://acme.cluster-abc123.us-east-1.docdb.amazonaws.com:27017
				user: flow_capture
				password_sops: ENC[AES256_GCM,data:abc,type:str]
				database: shop
				batchAndChangeStream: false
				pollSchedule: 6h
				sops:
					gcp_kms:
						- resource_id: projects/example/locations/us-central1/keyRings/example/cryptoKeys/example
							created_at: "2026-01-01T00:00:00Z"
							enc: <encrypted>
					lastmodified: "2026-01-01T00:00:00Z"
					mac: ENC[AES256_GCM,data:def,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				_meta:
					path: [shop, orders]
				database: shop
				collection: orders
				captureMode: Batch Incremental
				cursorField: updatedAt
				pollSchedule: 15m
			target: acmeCo/shop/orders
			backfill: 1
		- resource:
				_meta:
					path: [shop, countries]
				database: shop
				collection: countries
				captureMode: Batch Snapshot
				cursorField: _id
			target: acmeCo/shop/countries
	shards:
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

// documentDBTunnelLive is an Amazon DocumentDB capture behind an SSH tunnel whose host keys are set, a setting the
// connector schema has and the MongoDB spec doesn't map.
const documentDBTunnelLive string = `
	endpoint:
		connector:
			image: ghcr.io/estuary/source-amazon-documentdb:v3
			config:
				address: mongodb://acme.cluster-abc123.us-east-1.docdb.amazonaws.com:27017
				user: flow_capture
				password_sops: ENC[AES256_GCM,data:abc,type:str]
				networkTunnel:
					sshForwarding:
						sshEndpoint: ssh://ec2-user@bastion.example.com:22
						privateKey_sops: ENC[AES256_GCM,data:ghi,type:str]
						knownHosts: bastion.example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample
				sops:
					mac: ENC[AES256_GCM,data:def,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				_meta:
					path: [shop, orders]
				database: shop
				collection: orders
				captureMode: Batch Snapshot
				cursorField: _id
			target: acmeCo/shop/orders
	expectPubId: "11:22:33:44:55:66:77:88"
`

func documentDBSpec(discovers bool, config mongodb.Config, bindings ...capture.Binding[mongodb.Resource]) CaptureSpec[mongodb.Config, mongodb.Resource] {
	var spec CaptureSpec[mongodb.Config, mongodb.Resource] = CaptureSpec[mongodb.Config, mongodb.Resource]{
		Endpoint: endpoint.ConnectorSpec[mongodb.Config]{Connector: endpoint.Connector[mongodb.Config]{Image: mongodb.ImageAmazonDocumentDB, Config: config}},
		Bindings: bindings,
	}

	if discovers {
		spec.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true, EvolveIncompatibleCollections: true}
	}

	return spec
}

func TestDocumentDBSamplesRoundTrip(test *testing.T) {
	assertCaptureSamplesRoundTrip[mongodb.Config, mongodb.Resource](test, documentDBConnector, "../../catalog/capture/mongodb/testdata")
}

// The MongoDB spec requires password, so the configuration of an imported capture sets it: the first plan after the
// import shows only its hash, and the apply publishes the endpoint with the configured password, leaving the bindings
// as they are live.
func TestDocumentDBImportPlansOnlyItsPassword(test *testing.T) {
	var config mongodb.Config = mongodb.Config{
		Address:      "mongodb://acme.cluster-abc123.us-east-1.docdb.amazonaws.com:27017",
		User:         "flow_capture",
		Password:     "<secret>",
		Database:     "shop",
		PollSchedule: "6h",
	}

	entry := newConnectorCapture[mongodb.Config, mongodb.Resource](test, documentDBConnector, documentDBLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/shop/source-amazon-documentdb", false)

	entry.assertPlansOnlySecretHashes(test, "acmeCo/shop", documentDBSpec(true, config), state, configAttributePath("password"))

	var live []any = catalog.Bindings(entry.backend.live)

	published, _ := entry.apply(test, "acmeCo/shop", documentDBSpec(true, config,
		capture.Binding[mongodb.Resource]{
			Resource: mongodb.Resource{Database: "shop", Collection: "orders", CaptureMode: mongodb.CaptureModeBatchIncremental, CursorField: "updatedAt", PollSchedule: "15m"},
			Target:   "acmeCo/shop/orders",
		},
		capture.Binding[mongodb.Resource]{
			Resource: mongodb.Resource{Database: "shop", Collection: "countries", CaptureMode: mongodb.CaptureModeBatchSnapshot, CursorField: "_id"},
			Target:   "acmeCo/shop/countries",
		},
	), state)

	if !reflect.DeepEqual(catalog.Bindings(published), live) {
		test.Errorf("published bindings = %v, want the live ones, %v", catalog.Bindings(published), live)
	}

	var want map[string]any = map[string]any{
		"address":      "mongodb://acme.cluster-abc123.us-east-1.docdb.amazonaws.com:27017",
		"user":         "flow_capture",
		"password":     "<secret>",
		"database":     "shop",
		"pollSchedule": "6h",
	}

	if config := utils.Lookup(published, catalog.EndpointKey, catalog.ConnectorKey, catalog.ConfigKey); !reflect.DeepEqual(config, want) {
		test.Errorf("published config = %v, want the live one with the configured password, %v", config, want)
	}
}

// The import names the SSH host key settings the spec doesn't map, and the first apply, which publishes the endpoint
// for its required secrets, publishes it without them.
func TestDocumentDBImportNamesTheHostKeysItDoesNotManage(test *testing.T) {
	var ctx context.Context = context.Background()
	var config mongodb.Config = mongodb.Config{
		Address:  "mongodb://acme.cluster-abc123.us-east-1.docdb.amazonaws.com:27017",
		User:     "flow_capture",
		Password: "<secret>",
		NetworkTunnel: &endpoint.NetworkTunnel{SSHForwarding: &endpoint.SSHForwarding{
			SSHEndpoint: "ssh://ec2-user@bastion.example.com:22",
			PrivateKey:  "<private-key>",
		}},
	}

	entry := newConnectorCapture[mongodb.Config, mongodb.Resource](test, documentDBConnector, documentDBTunnelLive)

	var importResp resource.ImportStateResponse = resource.ImportStateResponse{
		State: tfsdk.State{Schema: entry.codec.Schema(), Raw: tftypes.NewValue(entry.codec.valueType, nil)},
	}

	entry.ImportState(ctx, resource.ImportStateRequest{ID: "acmeCo/shop/source-amazon-documentdb"}, &importResp)

	if warnings := importResp.Diagnostics.Warnings(); importResp.Diagnostics.HasError() || len(warnings) != 1 || !strings.Contains(warnings[0].Detail(), "knownHosts") {
		test.Fatalf("import diagnostics = %v, want one warning naming knownHosts", importResp.Diagnostics)
	}

	var state tftypes.Value = entry.importState(test, "acmeCo/shop/source-amazon-documentdb", true)

	entry.assertPlansOnlySecretHashes(test, "acmeCo/shop", documentDBSpec(false, config), state,
		configAttributePath("password"),
		configAttributePath("network_tunnel", "ssh_forwarding", "private_key"),
	)

	published, _ := entry.apply(test, "acmeCo/shop", documentDBSpec(false, config), state)

	var want map[string]any = map[string]any{"sshEndpoint": "ssh://ec2-user@bastion.example.com:22", "privateKey": "<private-key>"}

	if forwarding := utils.Lookup(published, catalog.EndpointKey, catalog.ConnectorKey, catalog.ConfigKey, "networkTunnel", "sshForwarding"); !reflect.DeepEqual(forwarding, want) {
		test.Errorf("published sshForwarding = %v, want the configured tunnel without the host keys, %v", forwarding, want)
	}
}
