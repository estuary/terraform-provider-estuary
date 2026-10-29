package postgres

import (
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/sqlcapture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/sslmode"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunCaptureSuite[Spec](test, spectest.Connector{Image: ImageAmazonAurora, Name: ConnectorNameAmazonAurora}, spectest.Connector{Image: ImageAmazonRDS, Name: ConnectorNameAmazonRDS})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, Resource](test, "password", "primary_key")
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, StatementTimeouts, auth.DatabaseAuthTypes, sslmode.PostgresModes, sqlcapture.BackfillModes)
}

func TestSampleAuthVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		authType  auth.DatabaseAuthType
		sshTunnel bool
	}{
		"acmeCo/aurora/postgres-password":   {auth.DatabaseAuthTypeUserPassword, false},
		"acmeCo/aurora/postgres-iam":        {auth.DatabaseAuthTypeAWSIAM, false},
		"acmeCo/aurora/postgres-ssh-tunnel": {auth.DatabaseAuthTypeUserPassword, true},
		"acmeCo/aurora/postgres-gcp-iam":    {auth.DatabaseAuthTypeGCPIAM, false},
		"acmeCo/aurora/postgres-azure-iam":  {auth.DatabaseAuthTypeAzureIAM, false},
		"acmeCo/rds/postgres-password":      {auth.DatabaseAuthTypeUserPassword, false},
		"acmeCo/rds/postgres-iam":           {auth.DatabaseAuthTypeAWSIAM, false},
		"acmeCo/rds/postgres-ssh-tunnel":    {auth.DatabaseAuthTypeUserPassword, true},
	}

	for name, expected := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			if !found {
				test.Fatal("capture missing from sample")
			}

			var config Config = spec.Endpoint.Connector.Config

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

			if config.Database != "postgres" || config.User != "flow_capture" {
				test.Errorf("database = %q, user = %q, want postgres and flow_capture", config.Database, config.User)
			}
		})
	}
}

func TestSampleLegacyPassword(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	var config Config = specs["acmeCo/aurora/postgres-legacy-password"].Endpoint.Connector.Config

	if config.Credentials != nil {
		test.Errorf("credentials = %+v, want unset for the legacy shape", config.Credentials)
	}

	if config.Password != "<secret>" {
		test.Errorf("legacy password = %q, want <secret>", config.Password)
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/aurora/postgres-all-options"]

	var config Config = spec.Endpoint.Connector.Config

	expectedFilters := DiscoveryFilters{
		IncludeSchemas:            []string{"public", "sales"},
		ExcludeSchemas:            []string{"audit"},
		TablePatterns:             []string{"public.orders*", "customers"},
		DiscoverUnpublishedTables: true,
	}

	expectedAdvanced := Advanced{
		PublicationName:           "estuary_publication",
		SlotName:                  "estuary_slot",
		WatermarksTable:           "public.estuary_watermarks",
		SkipBackfills:             "public.events,public.logs",
		BackfillChunkSize:         10000,
		SSLMode:                   sslmode.PostgresVerifyFull,
		DiscoverSchemas:           []string{"public"},
		MinimumBackfillXID:        "123456",
		ReadOnlyCapture:           true,
		CaptureAsPartitions:       true,
		DiscoverUnpublishedTables: true,
		DiscoverOnlyPublished:     true,
		SourceTag:                 "aurora-primary",
		RediscoveryInterval:       "1h",
		FeatureFlags:              "emit_sourced_schemas",
		StatementTimeout:          StatementTimeout5m,
	}

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	if !config.HistoryMode || config.Database != "analytics" {
		test.Errorf("historyMode = %v, database = %q", config.HistoryMode, config.Database)
	}

	if config.DiscoveryFilters == nil || !reflect.DeepEqual(*config.DiscoveryFilters, expectedFilters) {
		test.Errorf("discoveryFilters = %+v, want %+v", config.DiscoveryFilters, expectedFilters)
	}

	if config.Advanced == nil || !reflect.DeepEqual(*config.Advanced, expectedAdvanced) {
		test.Errorf("advanced = %+v, want %+v", config.Advanced, expectedAdvanced)
	}

	if config.NetworkTunnel == nil || config.NetworkTunnel.SSHForwarding == nil || !strings.HasPrefix(config.NetworkTunnel.SSHForwarding.PrivateKey, "-----BEGIN RSA PRIVATE KEY-----\n") {
		test.Errorf("networkTunnel = %+v, want an SSH forwarding with a multiline private key", config.NetworkTunnel)
	}
}

func TestSampleBindings(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	spectest.AssertSQLCaptureBindings(test, specs["acmeCo/aurora/postgres-all-options"].Bindings)
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var authTypes *spectest.EnumSamples[auth.DatabaseAuthType] = spectest.NewEnumSamples(test, "auth_type", auth.DatabaseAuthTypes)
	var backfillModes *spectest.EnumSamples[sqlcapture.BackfillMode] = spectest.NewEnumSamples(test, "backfill mode", sqlcapture.BackfillModes)
	var sslModes *spectest.EnumSamples[sslmode.Postgres] = spectest.NewEnumSamples(test, "sslmode", sslmode.PostgresModes)
	var statementTimeouts *spectest.EnumSamples[StatementTimeout] = spectest.NewEnumSamples(test, "statement_timeout", StatementTimeouts)

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			if config.Credentials != nil {
				authTypes.Add(name, config.Credentials.AuthType)
			}

			if config.Advanced != nil {
				if config.Advanced.SSLMode != "" {
					sslModes.Add(name, config.Advanced.SSLMode)
				}

				statementTimeouts.Add(name, config.Advanced.StatementTimeout)
			}

			for _, binding := range spec.Bindings {
				backfillModes.Add(name, binding.Resource.Mode)
			}
		}
	}

	authTypes.AssertAllSampled()

	backfillModes.AssertAllSampled()
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"timezone is a MySQL connector field": {
			Key: "timezone",
			Content: `
				captures:
					acmeCo/aurora/timezone:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-amazon-aurora-postgres:v3
								config:
									address: my-cluster.cluster-123456789.us-east-1.rds.amazonaws.com:5432
									user: flow_capture
									database: postgres
									timezone: UTC
						bindings: []
			`,
		},
		"misspelled advanced key": {
			Key: "publication_name",
			Content: `
				captures:
					acmeCo/aurora/typo:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-amazon-aurora-postgres:v3
								config:
									address: my-cluster.cluster-123456789.us-east-1.rds.amazonaws.com:5432
									user: flow_capture
									database: postgres
									advanced:
										publication_name: estuary_publication
						bindings: []
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}
