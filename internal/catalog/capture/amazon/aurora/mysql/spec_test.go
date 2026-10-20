package mysql

import (
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/sqlcapture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunCaptureSuite[Spec](test, Image)
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, Resource](test, "password", "primary_key")
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

			switch expected.authType {
			case auth.DatabaseAuthTypeUserPassword:
				if config.Credentials.Password != "<secret>" || config.Credentials.AWSRoleARN != "" {
					test.Errorf("password credentials = %+v, want password and no role", config.Credentials)
				}
			case auth.DatabaseAuthTypeAWSIAM:
				if config.Credentials.AWSRoleARN == "" || config.Credentials.AWSRegion != "us-east-1" || config.Credentials.Password != "" {
					test.Errorf("IAM credentials = %+v, want role and region and no password", config.Credentials)
				}
			}

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

	if advanced.SSLMode != SSLModeVerifyCA || !strings.HasPrefix(advanced.SSLServerCA, "-----BEGIN CERTIFICATE-----\n") || advanced.SSLClientCert == "" || advanced.SSLClientKey == "" {
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

	var bindings []Binding = specs["acmeCo/aurora/mysql-all-options"].Bindings

	expectations := []struct {
		stream   string
		mode     sqlcapture.BackfillMode
		priority int
		backfill int
		disable  bool
	}{
		{"orders", sqlcapture.BackfillModeAutomatic, 0, 0, false},
		{"customers", sqlcapture.BackfillModePrecise, 10, 2, false},
		{"events", sqlcapture.BackfillModeOnlyChanges, -1, 0, false},
		{"logs", sqlcapture.BackfillModeWithoutKey, 0, 0, true},
		{"invoices", sqlcapture.BackfillModeNormal, 0, 0, false},
	}

	if len(bindings) != len(expectations) {
		test.Fatalf("got %d bindings, want %d", len(bindings), len(expectations))
	}

	for index, expected := range expectations {
		var binding Binding = bindings[index]

		if binding.Resource.Stream != expected.stream || binding.Resource.Mode != expected.mode || binding.Resource.Priority != expected.priority {
			test.Errorf("binding %d resource = %+v, want stream %q mode %q priority %d", index, binding.Resource, expected.stream, expected.mode, expected.priority)
		}

		if binding.Backfill != expected.backfill || binding.Disable != expected.disable {
			test.Errorf("binding %d backfill = %d, disable = %v, want %d and %v", index, binding.Backfill, binding.Disable, expected.backfill, expected.disable)
		}
	}

	if bindings[2].Resource.Advanced == nil || bindings[2].Resource.Advanced.AdditionalBackfillFilter != "created_at > '2026-01-01'" {
		test.Errorf("events resource advanced = %+v, want the additional backfill filter", bindings[2].Resource.Advanced)
	}

	if !slices.Equal(bindings[3].Resource.PrimaryKey, []string{"id"}) {
		test.Errorf("logs primary_key = %v, want [id]", bindings[3].Resource.PrimaryKey)
	}
}

func TestEnumListsCoverSamples(test *testing.T) {
	var seenAuthTypes map[auth.DatabaseAuthType]bool = map[auth.DatabaseAuthType]bool{}

	var seenBackfillModes map[sqlcapture.BackfillMode]bool = map[sqlcapture.BackfillMode]bool{}

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			if config.Credentials != nil {
				if !slices.Contains(auth.DatabaseAuthTypes, config.Credentials.AuthType) {
					test.Errorf("%s: auth_type %q is not in AuthTypes", name, config.Credentials.AuthType)
				}

				seenAuthTypes[config.Credentials.AuthType] = true
			}

			if config.Advanced != nil {
				if config.Advanced.SSLMode != "" && !slices.Contains(SSLModes, config.Advanced.SSLMode) {
					test.Errorf("%s: sslmode %q is not in SSLModes", name, config.Advanced.SSLMode)
				}

				if !slices.Contains(StatementTimeouts, config.Advanced.StatementTimeout) {
					test.Errorf("%s: statement_timeout %q is not in StatementTimeouts", name, config.Advanced.StatementTimeout)
				}
			}

			for _, binding := range spec.Bindings {
				if !slices.Contains(sqlcapture.BackfillModes, binding.Resource.Mode) {
					test.Errorf("%s: mode %q is not in BackfillModes", name, binding.Resource.Mode)
				}

				seenBackfillModes[binding.Resource.Mode] = true
			}
		}
	}

	for _, authType := range []auth.DatabaseAuthType{auth.DatabaseAuthTypeUserPassword, auth.DatabaseAuthTypeAWSIAM} {
		if !seenAuthTypes[authType] {
			test.Errorf("auth_type %q has no sample", authType)
		}
	}

	for _, mode := range sqlcapture.BackfillModes {
		if !seenBackfillModes[mode] {
			test.Errorf("backfill mode %q has no sample", mode)
		}
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]struct {
		key     string
		content string
	}{
		"publicationName is a PostgreSQL connector field": {
			key: "publicationName",
			content: `
				captures:
					acmeCo/aurora/publication:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-amazon-aurora-mysql:v3
								config:
									address: my-cluster.cluster-abc123xyz.us-east-1.rds.amazonaws.com:3306
									user: flow_capture
									advanced:
										publicationName: flow_publication
						bindings: []
			`,
		},
		"database is not a MySQL connector field": {
			key: "database",
			content: `
				captures:
					acmeCo/aurora/database:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-amazon-aurora-mysql:v3
								config:
									address: my-cluster.cluster-abc123xyz.us-east-1.rds.amazonaws.com:3306
									user: flow_capture
									database: shop
						bindings: []
			`,
		},
	}

	for name, document := range documents {
		test.Run(name, func(test *testing.T) {
			_, err := spectest.ParseCaptures[Spec]([]byte(document.content))

			if err == nil || !strings.Contains(err.Error(), "field "+document.key+" not found") {
				test.Fatalf("error = %v, want strict decoding to reject the %s key", err, document.key)
			}
		})
	}
}
