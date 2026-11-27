package builder

import (
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/mysqlbatch"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/redshiftbatch"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/sqlbatch"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/sslmode"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var mysqlBatchConnector Connector = Connector{
	TypeName:      "source_mysql_batch",
	Image:         mysqlbatch.Image,
	ConnectorName: mysqlbatch.ConnectorName,
}

var redshiftBatchConnector Connector = Connector{
	TypeName:      "source_redshift_batch",
	Image:         redshiftbatch.Image,
	ConnectorName: redshiftbatch.ConnectorName,
}

// sqlBatchLiveBindings are the bindings of a batch SQL capture as discovery and the dashboard publish them: a table
// with a cursor, a full-refresh table with its own schedule, and a custom query, with their resource paths.
const sqlBatchLiveBindings string = `
	bindings:
		- resource:
				_meta:
					path: [orders]
				name: orders
				schema: shop
				table: orders
				cursor: [updated_at]
			target: acmeCo/shop/orders
			backfill: 1
		- resource:
				_meta:
					path: [countries]
				name: countries
				schema: shop
				table: countries
				poll: daily at 02:00Z
			target: acmeCo/shop/countries
		- resource:
				_meta:
					path: [revenue_by_day]
				name: revenue_by_day
				cursor: [day]
				template: SELECT day, SUM(total) AS revenue FROM shop.orders GROUP BY day
			target: acmeCo/shop/revenue_by_day
	shards:
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

// mysqlBatchLive is a MySQL batch capture as the dashboard publishes it: a sealed password and explicit defaults.
const mysqlBatchLive string = `
	autoDiscover:
		addNewBindings: true
		evolveIncompatibleCollections: true
	endpoint:
		connector:
			image: ghcr.io/estuary/source-mysql-batch:v1
			config:
				address: mysql.example.com:3306
				user: flow_capture
				password_sops: ENC[AES256_GCM,data:abc,type:str]
				advanced:
					poll: 6h
					discover_schemas: []
					discover_views: false
					sslmode: required
				sops:
					gcp_kms:
						- resource_id: projects/example/locations/us-central1/keyRings/example/cryptoKeys/example
							created_at: "2026-01-01T00:00:00Z"
							enc: <encrypted>
					lastmodified: "2026-01-01T00:00:00Z"
					mac: ENC[AES256_GCM,data:def,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
` + sqlBatchLiveBindings

// redshiftBatchLive is a Redshift batch capture as the dashboard publishes it: a sealed password and its database.
const redshiftBatchLive string = `
	autoDiscover:
		addNewBindings: true
		evolveIncompatibleCollections: true
	endpoint:
		connector:
			image: ghcr.io/estuary/source-redshift-batch:v1
			config:
				address: acme.abc123.us-east-1.redshift.amazonaws.com:5439
				user: flow_capture
				password_sops: ENC[AES256_GCM,data:abc,type:str]
				database: analytics
				advanced:
					discover_views: true
				sops:
					mac: ENC[AES256_GCM,data:def,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
` + sqlBatchLiveBindings

// redshiftBatchTunnelLive is a Redshift batch capture connecting through an SSH tunnel, with the sealed private key
// of the tunnel and the host keys of its server.
const redshiftBatchTunnelLive string = `
	endpoint:
		connector:
			image: ghcr.io/estuary/source-redshift-batch:v1
			config:
				address: acme.abc123.us-east-1.redshift.amazonaws.com:5439
				user: flow_capture
				password_sops: ENC[AES256_GCM,data:abc,type:str]
				database: analytics
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
					path: [orders]
				name: orders
				schema: shop
				table: orders
			target: acmeCo/shop/orders
	expectPubId: "11:22:33:44:55:66:77:88"
`

// sqlBatchBindings declares the bindings of sqlBatchLiveBindings.
var sqlBatchBindings []capture.Binding[sqlbatch.Resource] = []capture.Binding[sqlbatch.Resource]{
	{Resource: sqlbatch.Resource{Name: "orders", Schema: "shop", Table: "orders", Cursor: []string{"updated_at"}}, Target: "acmeCo/shop/orders"},
	{Resource: sqlbatch.Resource{Name: "countries", Schema: "shop", Table: "countries", Poll: "daily at 02:00Z"}, Target: "acmeCo/shop/countries"},
	{Resource: sqlbatch.Resource{Name: "revenue_by_day", Cursor: []string{"day"}, Template: "SELECT day, SUM(total) AS revenue FROM shop.orders GROUP BY day"}, Target: "acmeCo/shop/revenue_by_day"},
}

func sqlBatchSpec[C any](image string, discovers bool, config C, bindings ...capture.Binding[sqlbatch.Resource]) CaptureSpec[C, sqlbatch.Resource] {
	var spec CaptureSpec[C, sqlbatch.Resource] = CaptureSpec[C, sqlbatch.Resource]{
		Endpoint: endpoint.ConnectorSpec[C]{Connector: endpoint.Connector[C]{Image: image, Config: config}},
		Bindings: bindings,
	}

	if discovers {
		spec.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true, EvolveIncompatibleCollections: true}
	}

	return spec
}

func TestSQLBatchSamplesRoundTrip(test *testing.T) {
	assertCaptureSamplesRoundTrip[mysqlbatch.Config, mysqlbatch.Resource](test, mysqlBatchConnector, "../../catalog/capture/mysqlbatch/testdata")
	assertCaptureSamplesRoundTrip[redshiftbatch.Config, redshiftbatch.Resource](test, redshiftBatchConnector, "../../catalog/capture/redshiftbatch/testdata")
}

func TestMySQLBatchImportPlansNoChanges(test *testing.T) {
	var config mysqlbatch.Config = mysqlbatch.Config{
		Connection: sqlbatch.Connection{Address: "mysql.example.com:3306", User: "flow_capture"},
		Advanced: &mysqlbatch.Advanced{
			Advanced: sqlbatch.Advanced{Poll: "6h"},
			MySQLTLS: sslmode.MySQLTLS{SSLMode: sslmode.MySQLRequired},
		},
	}

	entry := newConnectorCapture[mysqlbatch.Config, mysqlbatch.Resource](test, mysqlBatchConnector, mysqlBatchLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/shop/source-mysql-batch", false)

	entry.assertPlansNoChanges(test, "acmeCo/shop", sqlBatchSpec(mysqlbatch.Image, true, config), state)
	entry.assertAppliesWithoutPublishing(test, "acmeCo/shop", sqlBatchSpec(mysqlbatch.Image, true, config, sqlBatchBindings...), state)
}

func TestRedshiftBatchImportPlansNoChanges(test *testing.T) {
	var config redshiftbatch.Config = redshiftbatch.Config{
		Connection: sqlbatch.Connection{Address: "acme.abc123.us-east-1.redshift.amazonaws.com:5439", User: "flow_capture"},
		Database:   "analytics",
		Advanced:   &redshiftbatch.Advanced{Advanced: sqlbatch.Advanced{DiscoverViews: true}},
	}

	entry := newConnectorCapture[redshiftbatch.Config, redshiftbatch.Resource](test, redshiftBatchConnector, redshiftBatchLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/shop/source-redshift-batch", false)

	entry.assertPlansNoChanges(test, "acmeCo/shop", sqlBatchSpec(redshiftbatch.Image, true, config), state)
	entry.assertAppliesWithoutPublishing(test, "acmeCo/shop", sqlBatchSpec(redshiftbatch.Image, true, config, sqlBatchBindings...), state)
}

// The private key of the shared SSH tunnel block is a required attribute, so a capture behind a tunnel can't leave
// its secrets out: the first plan after the import shows their hashes, and the apply publishes the endpoint with the
// configured secrets, leaving the bindings as they are live.
func TestSQLBatchImportBehindAnSSHTunnelPlansOnlyItsSecrets(test *testing.T) {
	var config redshiftbatch.Config = redshiftbatch.Config{
		Connection: sqlbatch.Connection{Address: "acme.abc123.us-east-1.redshift.amazonaws.com:5439", User: "flow_capture", Password: "<secret>"},
		Database:   "analytics",
		NetworkTunnel: &sqlbatch.NetworkTunnel{SSHForwarding: &sqlbatch.SSHForwarding{
			SSHForwarding: endpoint.SSHForwarding{SSHEndpoint: "ssh://ec2-user@bastion.example.com:22", PrivateKey: "<private-key>"},
			KnownHosts:    "bastion.example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample",
		}},
	}

	entry := newConnectorCapture[redshiftbatch.Config, redshiftbatch.Resource](test, redshiftBatchConnector, redshiftBatchTunnelLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/shop/source-redshift-batch", false)

	entry.assertPlansOnlySecretHashes(test, "acmeCo/shop", sqlBatchSpec(redshiftbatch.Image, false, config), state,
		configAttributePath("password"),
		configAttributePath("network_tunnel", "ssh_forwarding", "private_key"),
	)

	var live []any = catalog.Bindings(entry.backend.live)

	published, _ := entry.apply(test, "acmeCo/shop", sqlBatchSpec(redshiftbatch.Image, false, config, capture.Binding[sqlbatch.Resource]{
		Resource: sqlbatch.Resource{Name: "orders", Schema: "shop", Table: "orders"},
		Target:   "acmeCo/shop/orders",
	}), state)

	if published == nil || !reflect.DeepEqual(catalog.Bindings(published), live) {
		test.Errorf("published bindings = %v, want the live ones, %v", catalog.Bindings(published), live)
	}
}

func TestSQLBatchSecretsAreWriteOnly(test *testing.T) {
	var tunnel *sqlbatch.NetworkTunnel = &sqlbatch.NetworkTunnel{SSHForwarding: &sqlbatch.SSHForwarding{
		SSHForwarding: endpoint.SSHForwarding{SSHEndpoint: "ssh://ec2-user@bastion.example.com:22", PrivateKey: "<private-key>"},
	}}

	mysqlCodec, err := newCodec[captureDocument[mysqlbatch.Config, mysqlbatch.Resource]](mysqlBatchConnector, captureType[mysqlbatch.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	mysqlConfig, err := mysqlCodec.Value("shop", sqlBatchSpec(mysqlbatch.Image, false, mysqlbatch.Config{
		Connection:    sqlbatch.Connection{Address: "mysql.example.com:3306", User: "flow_capture", Password: "<secret>"},
		NetworkTunnel: tunnel,
		Advanced: &mysqlbatch.Advanced{MySQLTLS: sslmode.MySQLTLS{
			SSLMode:       sslmode.MySQLVerifyCA,
			SSLServerCA:   "<server-ca>",
			SSLClientCert: "<client-cert>",
			SSLClientKey:  "<client-key>",
		}},
	}))

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, mysqlCodec.secrets, mysqlConfig,
		configAttributePath("password"),
		configAttributePath("network_tunnel", "ssh_forwarding", "private_key"),
		configAttributePath("advanced", "ssl_server_ca"),
		configAttributePath("advanced", "ssl_client_cert"),
		configAttributePath("advanced", "ssl_client_key"),
	)

	redshiftCodec, err := newCodec[captureDocument[redshiftbatch.Config, redshiftbatch.Resource]](redshiftBatchConnector, captureType[redshiftbatch.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	redshiftConfig, err := redshiftCodec.Value("shop", sqlBatchSpec(redshiftbatch.Image, false, redshiftbatch.Config{
		Connection:    sqlbatch.Connection{Address: "acme.abc123.us-east-1.redshift.amazonaws.com:5439", User: "flow_capture", Password: "<secret>"},
		Database:      "analytics",
		NetworkTunnel: tunnel,
	}))

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, redshiftCodec.secrets, redshiftConfig,
		configAttributePath("password"),
		configAttributePath("network_tunnel", "ssh_forwarding", "private_key"),
	)
}
