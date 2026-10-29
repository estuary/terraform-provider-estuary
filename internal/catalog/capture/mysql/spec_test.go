package mysql

import (
	"slices"
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
	spectest.AssertEnumTags[Spec](test, StatementTimeouts, auth.DatabaseAuthTypes, sslmode.MySQLModes, sqlcapture.BackfillModes)
}

func TestSampleAuthVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		authType  auth.DatabaseAuthType
		sshTunnel bool
	}{
		"acmeCo/aurora/mysql-password":   {auth.DatabaseAuthTypeUserPassword, false},
		"acmeCo/aurora/mysql-iam":        {auth.DatabaseAuthTypeAWSIAM, false},
		"acmeCo/aurora/mysql-ssh-tunnel": {auth.DatabaseAuthTypeUserPassword, true},
		"acmeCo/aurora/mysql-gcp-iam":    {auth.DatabaseAuthTypeGCPIAM, false},
		"acmeCo/aurora/mysql-azure-iam":  {auth.DatabaseAuthTypeAzureIAM, false},
		"acmeCo/rds/mysql-password":      {auth.DatabaseAuthTypeUserPassword, false},
		"acmeCo/rds/mysql-iam":           {auth.DatabaseAuthTypeAWSIAM, false},
		"acmeCo/rds/mysql-ssh-tunnel":    {auth.DatabaseAuthTypeUserPassword, true},
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

			if config.User != "flow_capture" {
				test.Errorf("user = %q, want flow_capture", config.User)
			}
		})
	}
}

func TestSampleLegacyPassword(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	var config Config = specs["acmeCo/aurora/mysql-legacy-password"].Endpoint.Connector.Config

	if config.Credentials != nil {
		test.Errorf("credentials = %+v, want unset for the legacy shape", config.Credentials)
	}

	if config.Password != "<secret>" || config.Timezone != "America/New_York" {
		test.Errorf("legacy password = %q, timezone = %q", config.Password, config.Timezone)
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/aurora/mysql-all-options"]

	var config Config = spec.Endpoint.Connector.Config

	expectedFilters := DiscoveryFilters{
		IncludeSchemas: []string{"shop", "billing"},
		ExcludeSchemas: []string{"audit"},
		TablePatterns:  []string{"shop.orders*", "customers"},
	}

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	if !config.HistoryMode || config.Timezone != "+00:00" {
		test.Errorf("historyMode = %v, timezone = %q", config.HistoryMode, config.Timezone)
	}

	if config.DiscoveryFilters == nil ||
		!slices.Equal(config.DiscoveryFilters.IncludeSchemas, expectedFilters.IncludeSchemas) ||
		!slices.Equal(config.DiscoveryFilters.ExcludeSchemas, expectedFilters.ExcludeSchemas) ||
		!slices.Equal(config.DiscoveryFilters.TablePatterns, expectedFilters.TablePatterns) {
		test.Errorf("discoveryFilters = %+v, want %+v", config.DiscoveryFilters, expectedFilters)
	}

	if config.Advanced == nil {
		test.Fatal("advanced = nil, want every advanced option set")
	}

	var advanced Advanced = *config.Advanced

	if advanced.DBName != "shop" || !advanced.SkipBinlogRetentionCheck || advanced.NodeID != 1234 {
		test.Errorf("dbname = %q, skip_binlog_retention_check = %v, node_id = %d", advanced.DBName, advanced.SkipBinlogRetentionCheck, advanced.NodeID)
	}

	if advanced.SkipBackfills != "shop.events,shop.logs" || advanced.BackfillChunkSize != 10000 || !slices.Equal(advanced.DiscoverSchemas, []string{"shop"}) {
		test.Errorf("backfill options = %q / %d / %v", advanced.SkipBackfills, advanced.BackfillChunkSize, advanced.DiscoverSchemas)
	}

	if advanced.SourceTag != "aurora-writer" || advanced.RediscoveryInterval != "1h" || advanced.FeatureFlags != "tinyint1_as_bool" || advanced.StatementTimeout != StatementTimeout5m {
		test.Errorf("source_tag = %q, rediscovery_interval = %q, feature_flags = %q, statement_timeout = %q", advanced.SourceTag, advanced.RediscoveryInterval, advanced.FeatureFlags, advanced.StatementTimeout)
	}

	if advanced.SSLMode != sslmode.MySQLVerifyCA || !strings.HasPrefix(advanced.SSLServerCA, "-----BEGIN CERTIFICATE-----\n") || advanced.SSLClientCert == "" || advanced.SSLClientKey == "" {
		test.Errorf("TLS options = %q / %q / %q / %q", advanced.SSLMode, advanced.SSLServerCA, advanced.SSLClientCert, advanced.SSLClientKey)
	}

	if advanced.WatermarksTable != "flow.watermarks" || advanced.HeartbeatInterval != "60s" {
		test.Errorf("deprecated options = %q / %q", advanced.WatermarksTable, advanced.HeartbeatInterval)
	}

	if config.NetworkTunnel == nil || config.NetworkTunnel.SSHForwarding == nil || !strings.HasPrefix(config.NetworkTunnel.SSHForwarding.PrivateKey, "-----BEGIN RSA PRIVATE KEY-----\n") {
		test.Errorf("networkTunnel = %+v, want an SSH forwarding with a multiline private key", config.NetworkTunnel)
	}
}

func TestSampleBindings(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	spectest.AssertSQLCaptureBindings(test, specs["acmeCo/aurora/mysql-all-options"].Bindings)
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var authTypes *spectest.EnumSamples[auth.DatabaseAuthType] = spectest.NewEnumSamples(test, "auth_type", auth.DatabaseAuthTypes)
	var backfillModes *spectest.EnumSamples[sqlcapture.BackfillMode] = spectest.NewEnumSamples(test, "backfill mode", sqlcapture.BackfillModes)
	var sslModes *spectest.EnumSamples[sslmode.MySQL] = spectest.NewEnumSamples(test, "sslmode", sslmode.MySQLModes)
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
		"publicationName is a PostgreSQL connector field": {
			Key: "publicationName",
			Content: `
				captures:
					acmeCo/aurora/publication:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-amazon-aurora-mysql:v3
								config:
									address: my-cluster.cluster-123456789.us-east-1.rds.amazonaws.com:3306
									user: flow_capture
									advanced:
										publicationName: flow_publication
						bindings: []
			`,
		},
		"database is not a MySQL connector field": {
			Key: "database",
			Content: `
				captures:
					acmeCo/aurora/database:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-amazon-aurora-mysql:v3
								config:
									address: my-cluster.cluster-123456789.us-east-1.rds.amazonaws.com:3306
									user: flow_capture
									database: shop
						bindings: []
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}
