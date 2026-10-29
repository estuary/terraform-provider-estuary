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
	spectest.RunMaterializationSuite[Spec](test, spectest.Connector{Image: ImagePostgres, Name: ConnectorNamePostgres}, spectest.Connector{Image: ImageAmazonAurora, Name: ConnectorNameAmazonAurora}, spectest.Connector{Image: ImageAmazonRDS, Name: ConnectorNameAmazonRDS})
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, auth.DatabaseAuthTypes, sslmode.PostgresModes, materialization.DBTJobTriggerModes, materialization.OnIncompatibleSchemaChanges, materialization.TargetNamingStrategies)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		image     string
		authType  auth.DatabaseAuthType
		sshTunnel bool
	}{
		"acmeCo/postgres/password":     {ImagePostgres, auth.DatabaseAuthTypeUserPassword, false},
		"acmeCo/postgres/gcp-iam":      {ImagePostgres, auth.DatabaseAuthTypeGCPIAM, false},
		"acmeCo/postgres/azure-iam":    {ImagePostgres, auth.DatabaseAuthTypeAzureIAM, false},
		"acmeCo/postgres/ssh-tunnel":   {ImagePostgres, auth.DatabaseAuthTypeUserPassword, true},
		"acmeCo/aurora/postgres-iam":   {ImageAmazonAurora, auth.DatabaseAuthTypeAWSIAM, false},
		"acmeCo/rds/postgres-password": {ImageAmazonRDS, auth.DatabaseAuthTypeUserPassword, false},
		"acmeCo/rds/postgres-iam":      {ImageAmazonRDS, auth.DatabaseAuthTypeAWSIAM, false},
	}

	if len(specs) != len(expectations)+1 {
		test.Fatalf("got %d materializations, want %d plus the legacy password", len(specs), len(expectations))
	}

	for name, expected := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			if !found {
				test.Fatal("materialization missing from sample")
			}

			var config Config = spec.Endpoint.Connector.Config

			if spec.Endpoint.Connector.Image != expected.image {
				test.Errorf("image = %q, want %q", spec.Endpoint.Connector.Image, expected.image)
			}

			if config.Credentials == nil || config.Credentials.AuthType != expected.authType {
				test.Fatalf("credentials = %+v, want auth_type %q", config.Credentials, expected.authType)
			}

			spectest.AssertDatabaseCredentials(test, *config.Credentials)

			if (config.NetworkTunnel != nil) != expected.sshTunnel {
				test.Errorf("networkTunnel = %+v, want present = %v", config.NetworkTunnel, expected.sshTunnel)
			}

			if config.Password != "" {
				test.Errorf("legacy password = %q, want unset", config.Password)
			}
		})
	}
}

func TestSampleLegacyPassword(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	var config Config = specs["acmeCo/postgres/legacy-password"].Endpoint.Connector.Config

	if config.Credentials != nil {
		test.Errorf("credentials = %+v, want unset for the legacy shape", config.Credentials)
	}

	if config.Password != "<secret>" {
		test.Errorf("legacy password = %q, want <secret>", config.Password)
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

	spectest.AssertMaterializationResources(test, bindings, []Resource{
		{Table: "orders"},
		{Table: "customers", Schema: "crm", AdditionalTableCreateSQL: "CREATE INDEX customers_email ON crm.customers (email);"},
		{Table: "events", DeltaUpdates: true},
	})

	if bindings[2].Fields == nil || !slices.Equal(bindings[2].Fields.Exclude, []string{"payload"}) {
		test.Errorf("events fields = %+v, want exclude [payload]", bindings[2].Fields)
	}

	if _, found := bindings[2].Fields.Require["flow_document"]; !found || len(bindings[2].Fields.Require) != 1 {
		test.Errorf("events fields = %+v, want require [flow_document]", bindings[2].Fields)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var authTypes *spectest.EnumSamples[auth.DatabaseAuthType] = spectest.NewEnumSamples(test, "auth_type", auth.DatabaseAuthTypes)
	var sslModes *spectest.EnumSamples[sslmode.Postgres] = spectest.NewEnumSamples(test, "sslmode", sslmode.PostgresModes)

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			if config.Credentials == nil && config.Password == "" {
				test.Errorf("%s: has no credentials and no legacy password, which the connector rejects", name)
			}

			if config.Credentials != nil {
				authTypes.Add(name, config.Credentials.AuthType)
			}

			if config.Advanced != nil && config.Advanced.SSLMode != "" {
				sslModes.Add(name, config.Advanced.SSLMode)
			}
		}
	}

	authTypes.AssertAllSampled()
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
