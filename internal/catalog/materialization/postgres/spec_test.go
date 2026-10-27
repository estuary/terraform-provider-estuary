package postgres

import (
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/sslmode"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunMaterializationSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, auth.DatabaseAuthTypes, sslmode.PostgresModes, materialization.DBTJobTriggerModes, materialization.OnIncompatibleSchemaChanges, materialization.TargetNamingStrategies)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		authType       auth.DatabaseAuthType
		legacyPassword bool
		sshTunnel      bool
	}{
		"acmeCo/postgres/password":        {auth.DatabaseAuthTypeUserPassword, false, false},
		"acmeCo/postgres/aurora-iam":      {auth.DatabaseAuthTypeAWSIAM, false, false},
		"acmeCo/postgres/ssh-tunnel":      {auth.DatabaseAuthTypeUserPassword, false, true},
		"acmeCo/postgres/legacy-password": {"", true, false},
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

			if expected.legacyPassword && (config.Credentials != nil || config.Password == "") {
				test.Errorf("credentials = %+v, password = %q, want only the legacy password", config.Credentials, config.Password)
			}

			if !expected.legacyPassword && (config.Credentials == nil || config.Credentials.AuthType != expected.authType || config.Password != "") {
				test.Errorf("credentials = %+v, password = %q, want auth_type %q", config.Credentials, config.Password, expected.authType)
			}

			if (config.NetworkTunnel != nil) != expected.sshTunnel {
				test.Errorf("networkTunnel = %+v, want present = %v", config.NetworkTunnel, expected.sshTunnel)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var config Config = specs["acmeCo/analytics/materialize-postgres"].Endpoint.Connector.Config

	expectedAdvanced := Advanced{
		SSLMode:        sslmode.PostgresVerifyFull,
		NoFlowDocument: true,
		FeatureFlags:   "allow_existing_tables_for_new_bindings",
	}

	if config.Database != "analytics" || config.Schema != "estuary" || !config.HardDelete {
		test.Errorf("database = %q, schema = %q, hardDelete = %v", config.Database, config.Schema, config.HardDelete)
	}

	if config.DBTJobTrigger == nil || config.DBTJobTrigger.JobID != "12345" || config.DBTJobTrigger.Mode != materialization.DBTJobTriggerModeSkip {
		test.Errorf("dbt_job_trigger = %+v, want job 12345 in skip mode", config.DBTJobTrigger)
	}

	if config.Advanced == nil || *config.Advanced != expectedAdvanced {
		test.Errorf("advanced = %+v, want %+v", config.Advanced, expectedAdvanced)
	}

	if config.NetworkTunnel == nil || config.NetworkTunnel.SSHForwarding == nil || !strings.HasPrefix(config.NetworkTunnel.SSHForwarding.PrivateKey, "-----BEGIN RSA PRIVATE KEY-----\n") {
		test.Errorf("networkTunnel = %+v, want an SSH forwarding with a multiline private key", config.NetworkTunnel)
	}
}

func TestSampleBindings(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var bindings []Binding = specs["acmeCo/analytics/materialize-postgres"].Bindings

	expectations := []Resource{
		{Table: "orders"},
		{Table: "customers", Schema: "crm", AdditionalTableCreateSQL: "CREATE INDEX customers_email ON crm.customers (email);"},
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

	if bindings[2].Fields == nil || !slices.Equal(bindings[2].Fields.Exclude, []string{"payload"}) {
		test.Errorf("events fields = %+v, want exclude [payload]", bindings[2].Fields)
	}

	if _, found := bindings[2].Fields.Require["flow_document"]; !found || len(bindings[2].Fields.Require) != 1 {
		test.Errorf("events fields = %+v, want require [flow_document]", bindings[2].Fields)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var authTypes *spectest.EnumSamples[auth.DatabaseAuthType] = spectest.NewEnumSamples(test, "auth_type", auth.DatabaseAuthTypes)

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			if config.Credentials == nil && config.Password == "" {
				test.Errorf("%s: has no credentials and no legacy password, which the connector rejects", name)
			}

			if config.Credentials != nil {
				var credentials auth.DatabaseCredentials = *config.Credentials

				authTypes.Add(name, credentials.AuthType)

				if credentials.AuthType == auth.DatabaseAuthTypeUserPassword && credentials.Password == "" {
					test.Errorf("%s: UserPassword credentials need a password", name)
				}

				if credentials.AuthType == auth.DatabaseAuthTypeAWSIAM && (credentials.AWSRegion == "" || credentials.AWSRoleARN == "") {
					test.Errorf("%s: AWSIAM credentials need aws_region and aws_role_arn", name)
				}
			}

			if config.Advanced != nil && config.Advanced.SSLMode != "" && !slices.Contains(sslmode.PostgresModes, config.Advanced.SSLMode) {
				test.Errorf("%s: sslmode %q is not in sslmode.PostgresModes", name, config.Advanced.SSLMode)
			}
		}
	}

	authTypes.AssertSampled(auth.DatabaseAuthTypeUserPassword, auth.DatabaseAuthTypeAWSIAM)
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"timezone is a MySQL materialization field": {
			Key: "timezone",
			Content: `
				materializations:
					acmeCo/postgres/timezone:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-postgres:v5
								config:
									address: my-cluster.cluster-123456789.us-east-1.rds.amazonaws.com:5432
									user: flow_materialize
									timezone: UTC
									credentials:
										auth_type: UserPassword
										password: <secret>
						bindings: []
			`,
		},
		"stream is a SQL capture resource field": {
			Key: "stream",
			Content: `
				materializations:
					acmeCo/postgres/stream:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-postgres:v5
								config:
									address: my-cluster.cluster-123456789.us-east-1.rds.amazonaws.com:5432
									user: flow_materialize
									credentials:
										auth_type: UserPassword
										password: <secret>
						bindings:
							- resource:
									table: orders
									stream: orders
								source: acmeCo/orders
			`,
		},
	}

	spectest.AssertUnknownMaterializationKeys[Spec](test, documents)
}
