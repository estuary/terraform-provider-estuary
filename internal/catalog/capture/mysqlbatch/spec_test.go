package mysqlbatch

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/sqlbatch"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/sslmode"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunCaptureSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, Resource](test)
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, sslmode.MySQLModes)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		password  string
		sshTunnel bool
		bindings  []Resource
	}{
		"acmeCo/mysql-batch/tables": {"<secret>", false, []Resource{
			{Name: "orders", Schema: "shop", Table: "orders", Cursor: []string{"updated_at"}},
			{Name: "countries", Schema: "shop", Table: "countries"},
		}},
		"acmeCo/mysql-batch/ssh-tunnel": {"<secret>", true, []Resource{{Name: "orders", Schema: "shop", Table: "orders", Cursor: []string{"updated_at"}, Poll: "15m"}}},
		"acmeCo/mysql-batch/imported":   {"", false, []Resource{{Name: "orders", Schema: "shop", Table: "orders"}}},
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

			var want sqlbatch.Connection = sqlbatch.Connection{Address: "mysql.example.com:3306", User: "flow_capture", Password: expected.password}

			if config.Connection != want {
				test.Errorf("connection = %+v, want %+v", config.Connection, want)
			}

			if (config.NetworkTunnel != nil) != expected.sshTunnel {
				test.Errorf("networkTunnel = %+v, want present = %v", config.NetworkTunnel, expected.sshTunnel)
			}

			var resources []Resource

			for _, binding := range spec.Bindings {
				resources = append(resources, binding.Resource)
			}

			if !reflect.DeepEqual(resources, expected.bindings) {
				test.Errorf("resources = %+v, want %+v", resources, expected.bindings)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/mysql-batch/all-options"]

	var config Config = spec.Endpoint.Connector.Config

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	expectedCommon := sqlbatch.Advanced{
		Poll:            "6h",
		DiscoverSchemas: []string{"shop", "billing"},
		DiscoverViews:   true,
		SourceTag:       "primary",
		FeatureFlags:    "emit_sourced_schemas",
	}

	if config.Advanced == nil || !reflect.DeepEqual(config.Advanced.Advanced, expectedCommon) {
		test.Fatalf("advanced = %+v, want the shared options %+v", config.Advanced, expectedCommon)
	}

	if config.Advanced.DBName != "shop" || config.Advanced.SSLMode != sslmode.MySQLVerifyCA {
		test.Errorf("dbname = %q, sslmode = %q", config.Advanced.DBName, config.Advanced.SSLMode)
	}

	for label, certificate := range map[string]string{"ssl_server_ca": config.Advanced.SSLServerCA, "ssl_client_cert": config.Advanced.SSLClientCert, "ssl_client_key": config.Advanced.SSLClientKey} {
		if !strings.HasPrefix(certificate, "-----BEGIN ") {
			test.Errorf("%s = %q, want a multiline PEM value", label, certificate)
		}
	}

	if tunnel := config.NetworkTunnel; tunnel == nil || tunnel.SSHForwarding == nil || !strings.HasPrefix(tunnel.SSHForwarding.PrivateKey, "-----BEGIN RSA PRIVATE KEY-----\n") ||
		!strings.Contains(tunnel.SSHForwarding.KnownHosts, "ssh-ed25519") || !tunnel.SSHForwarding.SkipHostKeyVerification {
		test.Errorf("networkTunnel = %+v, want an SSH forwarding with a multiline private key and both host key settings", tunnel)
	}
}

func TestSampleBindings(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var bindings []Binding = specs["acmeCo/mysql-batch/all-options"].Bindings

	if len(bindings) != 3 {
		test.Fatalf("got %d bindings, want 3", len(bindings))
	}

	var orders Resource = Resource{Name: "orders", Schema: "shop", Table: "orders", Cursor: []string{"updated_at", "id"}, Poll: "15m"}
	var countries Resource = Resource{Name: "countries", Schema: "shop", Table: "countries", Poll: "daily at 02:00Z"}

	if !reflect.DeepEqual(bindings[0].Resource, orders) || !reflect.DeepEqual(bindings[1].Resource, countries) || bindings[1].Backfill != 1 {
		test.Errorf("table bindings = %+v and %+v, want %+v and %+v backfilled once", bindings[0], bindings[1], orders, countries)
	}

	var template Resource = bindings[2].Resource

	if template.Name != "revenue_by_day" || template.Schema != "" || template.Table != "" || !slices.Equal(template.Cursor, []string{"day"}) ||
		!strings.Contains(template.Template, "{{if not .IsFirstQuery}}") || !bindings[2].Disable {
		test.Errorf("template binding = %+v, want a disabled custom query with a cursor and no table", bindings[2])
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var modes *spectest.EnumSamples[sslmode.MySQL] = spectest.NewEnumSamples(test, "sslmode", sslmode.MySQLModes)
	var sampled map[string]bool = map[string]bool{}

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			sampled["ssh tunnel "+map[bool]string{true: "set", false: "unset"}[config.NetworkTunnel != nil]] = true

			if config.Advanced != nil && config.Advanced.SSLMode != "" {
				modes.Add(name, config.Advanced.SSLMode)
			}

			for _, binding := range spec.Bindings {
				var resource Resource = binding.Resource

				if (resource.Schema == "" || resource.Table == "") && resource.Template == "" {
					test.Errorf("%s: binding %s needs a schema and table, or a template", name, resource.Name)
				}

				sampled[map[bool]string{true: "template", false: "table"}[resource.Template != ""]+" binding"] = true
				sampled["cursor "+map[bool]string{true: "set", false: "unset"}[len(resource.Cursor) > 0]] = true
				sampled["poll "+map[bool]string{true: "set", false: "unset"}[resource.Poll != ""]] = true
			}
		}
	}

	modes.AssertSampled(sslmode.MySQLVerifyCA)

	for _, shape := range []string{"ssh tunnel set", "ssh tunnel unset", "template binding", "table binding", "cursor set", "cursor unset", "poll set", "poll unset"} {
		if !sampled[shape] {
			test.Errorf("no sample with %s", shape)
		}
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"database is a Redshift batch field": {
			Key: "database",
			Content: `
				captures:
					acmeCo/mysql-batch/database:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-mysql-batch:v1
								config:
									address: mysql.example.com:3306
									user: flow_capture
									password: <secret>
									database: shop
						bindings: []
			`,
		},
		"credentials is a field of the MySQL change data capture": {
			Key: "credentials",
			Content: `
				captures:
					acmeCo/mysql-batch/credentials:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-mysql-batch:v1
								config:
									address: mysql.example.com:3306
									user: flow_capture
									credentials:
										auth_type: UserPassword
										password: <secret>
						bindings: []
			`,
		},
		"namespace is a resource field of the MySQL change data capture": {
			Key: "namespace",
			Content: `
				captures:
					acmeCo/mysql-batch/namespace:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-mysql-batch:v1
								config:
									address: mysql.example.com:3306
									user: flow_capture
									password: <secret>
						bindings:
							- resource:
									name: orders
									namespace: shop
								target: acmeCo/mysql-batch/orders
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}
