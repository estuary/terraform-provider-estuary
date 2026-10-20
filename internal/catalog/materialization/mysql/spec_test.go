package mysql

import (
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunMaterializationSuite[Spec](test, Image)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		sslMode   SSLMode
		timezone  string
		sshTunnel bool
	}{
		"acmeCo/mysql/password":   {"", "", false},
		"acmeCo/mysql/verify-ca":  {SSLModeVerifyCA, "", false},
		"acmeCo/mysql/ssh-tunnel": {"", "", true},
		"acmeCo/mysql/timezone":   {"", "America/New_York", false},
	}

	if len(specs) != len(expectations) {
		test.Fatalf("got %d materializations, want %d", len(specs), len(expectations))
	}

	for name, expected := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			if !found {
				test.Fatal("materialization missing from sample")
			}

			var config Config = spec.Endpoint.Connector.Config

			var sslMode SSLMode

			if config.Advanced != nil {
				sslMode = config.Advanced.SSLMode
			}

			if config.User != "flow_materialize" || config.Password != "<secret>" || config.Database != "analytics" {
				test.Errorf("user = %q, password = %q, database = %q", config.User, config.Password, config.Database)
			}

			if sslMode != expected.sslMode || config.Timezone != expected.timezone {
				test.Errorf("sslmode = %q, timezone = %q, want %q and %q", sslMode, config.Timezone, expected.sslMode, expected.timezone)
			}

			if (config.NetworkTunnel != nil) != expected.sshTunnel {
				test.Errorf("networkTunnel = %+v, want present = %v", config.NetworkTunnel, expected.sshTunnel)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var config Config = specs["acmeCo/analytics/materialize-mysql"].Endpoint.Connector.Config

	if config.Timezone != "+00:00" || !config.HardDelete {
		test.Errorf("timezone = %q, hardDelete = %v", config.Timezone, config.HardDelete)
	}

	if config.DBTJobTrigger == nil || config.DBTJobTrigger.Mode != materialization.DBTJobTriggerModeIgnore {
		test.Errorf("dbt_job_trigger = %+v, want ignore mode", config.DBTJobTrigger)
	}

	if config.Advanced == nil || config.Advanced.SSLMode != SSLModeVerifyIdentity || !config.Advanced.NoFlowDocument {
		test.Fatalf("advanced = %+v, want verify_identity without the flow document", config.Advanced)
	}

	for field, value := range map[string]string{"ssl_server_ca": config.Advanced.SSLServerCA, "ssl_client_cert": config.Advanced.SSLClientCert, "ssl_client_key": config.Advanced.SSLClientKey} {
		if !strings.HasPrefix(value, "-----BEGIN ") {
			test.Errorf("%s = %q, want a multiline PEM block", field, value)
		}
	}
}

func TestSampleBindings(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var bindings []Binding = specs["acmeCo/analytics/materialize-mysql"].Bindings

	expectations := []Resource{
		{Table: "orders"},
		{Table: "customers", AdditionalTableCreateSQL: "CREATE INDEX customers_email ON customers (email);"},
		{Table: "events", DeltaUpdates: true},
	}

	if len(bindings) != len(expectations) {
		test.Fatalf("got %d bindings, want %d", len(bindings), len(expectations))
	}

	for index, expected := range expectations {
		if bindings[index].Resource != expected {
			test.Errorf("binding %d resource = %+v, want %+v", index, bindings[index].Resource, expected)
		}
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			if config.Address == "" || config.User == "" || config.Password == "" || config.Database == "" {
				test.Errorf("%s: address, user, password and database are all required", name)
			}

			if config.Advanced == nil {
				continue
			}

			var sslMode SSLMode = config.Advanced.SSLMode

			if sslMode != "" && !slices.Contains(SSLModes, sslMode) {
				test.Errorf("%s: sslmode %q is not in SSLModes", name, sslMode)
			}

			if (sslMode == SSLModeVerifyCA || sslMode == SSLModeVerifyIdentity) && config.Advanced.SSLServerCA == "" {
				test.Errorf("%s: sslmode %q requires ssl_server_ca", name, sslMode)
			}
		}
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]struct {
		key     string
		content string
	}{
		"credentials is a MySQL capture field": {
			key: "credentials",
			content: `
				materializations:
					acmeCo/mysql/credentials:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-mysql:v2
								config:
									address: my-cluster.cluster-abc123.us-east-1.rds.amazonaws.com:3306
									user: flow_materialize
									database: analytics
									credentials:
										auth_type: UserPassword
										password: <secret>
						bindings: []
			`,
		},
		"schema is a PostgreSQL resource field": {
			key: "schema",
			content: `
				materializations:
					acmeCo/mysql/schema:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-mysql:v2
								config:
									address: my-cluster.cluster-abc123.us-east-1.rds.amazonaws.com:3306
									user: flow_materialize
									password: <secret>
									database: analytics
						bindings:
							- resource:
									table: orders
									schema: crm
								source: acmeCo/orders
			`,
		},
	}

	for name, document := range documents {
		test.Run(name, func(test *testing.T) {
			_, err := spectest.ParseMaterializations[Spec]([]byte(document.content))

			if err == nil || !strings.Contains(err.Error(), "field "+document.key+" not found") {
				test.Fatalf("error = %v, want strict decoding to reject the %s key", err, document.key)
			}
		})
	}
}
