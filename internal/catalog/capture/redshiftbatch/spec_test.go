package redshiftbatch

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
	spectest.AssertEnumTags[Spec](test, sslmode.PostgresModes)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		password  string
		sshTunnel bool
		bindings  []Resource
	}{
		"acmeCo/redshift-batch/tables": {"<secret>", false, []Resource{
			{Name: "orders", Schema: "shop", Table: "orders", Cursor: []string{"updated_at"}},
			{Name: "countries", Schema: "shop", Table: "countries"},
		}},
		"acmeCo/redshift-batch/ssh-tunnel": {"<secret>", true, []Resource{{Name: "orders", Schema: "shop", Table: "orders", Cursor: []string{"updated_at"}, Poll: "15m"}}},
		"acmeCo/redshift-batch/imported":   {"", false, []Resource{{Name: "orders", Schema: "shop", Table: "orders"}}},
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

			var want sqlbatch.Connection = sqlbatch.Connection{Address: "acme.abc123.us-east-1.redshift.amazonaws.com:5439", User: "flow_capture", Password: expected.password}

			if config.Connection != want || config.Database != "analytics" {
				test.Errorf("connection = %+v, database = %q, want %+v and analytics", config.Connection, config.Database, want)
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

	var spec Spec = specs["acmeCo/redshift-batch/all-options"]

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

	if config.Database != "analytics" || config.Advanced.SSLMode != sslmode.PostgresVerifyFull {
		test.Errorf("database = %q, sslmode = %q", config.Database, config.Advanced.SSLMode)
	}

	if tunnel := config.NetworkTunnel; tunnel == nil || tunnel.SSHForwarding == nil || !strings.HasPrefix(tunnel.SSHForwarding.PrivateKey, "-----BEGIN RSA PRIVATE KEY-----\n") ||
		!strings.Contains(tunnel.SSHForwarding.KnownHosts, "ssh-ed25519") || !tunnel.SSHForwarding.SkipHostKeyVerification {
		test.Errorf("networkTunnel = %+v, want an SSH forwarding with a multiline private key and both host key settings", tunnel)
	}
}

func TestSampleBindings(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var bindings []Binding = specs["acmeCo/redshift-batch/all-options"].Bindings

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
	var modes *spectest.EnumSamples[sslmode.Postgres] = spectest.NewEnumSamples(test, "sslmode", sslmode.PostgresModes)
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

	modes.AssertSampled(sslmode.PostgresVerifyFull)

	for _, shape := range []string{"ssh tunnel set", "ssh tunnel unset", "template binding", "table binding", "cursor set", "cursor unset", "poll set", "poll unset"} {
		if !sampled[shape] {
			test.Errorf("no sample with %s", shape)
		}
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"dbname is a MySQL batch advanced option": {
			Key: "dbname",
			Content: `
				captures:
					acmeCo/redshift-batch/dbname:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-redshift-batch:v1
								config:
									address: acme.abc123.us-east-1.redshift.amazonaws.com:5439
									user: flow_capture
									password: <secret>
									database: analytics
									advanced:
										dbname: analytics
						bindings: []
			`,
		},
		"credentials is a field of the SQL change data captures": {
			Key: "credentials",
			Content: `
				captures:
					acmeCo/redshift-batch/credentials:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-redshift-batch:v1
								config:
									address: acme.abc123.us-east-1.redshift.amazonaws.com:5439
									user: flow_capture
									database: analytics
									credentials:
										auth_type: UserPassword
										password: <secret>
						bindings: []
			`,
		},
		"namespace is a resource field of the SQL change data captures": {
			Key: "namespace",
			Content: `
				captures:
					acmeCo/redshift-batch/namespace:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-redshift-batch:v1
								config:
									address: acme.abc123.us-east-1.redshift.amazonaws.com:5439
									user: flow_capture
									password: <secret>
									database: analytics
						bindings:
							- resource:
									name: orders
									namespace: shop
								target: acmeCo/redshift-batch/orders
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}
