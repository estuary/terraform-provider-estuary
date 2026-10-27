package postgres

import (
	"reflect"
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

			switch expected.authType {
			case auth.DatabaseAuthTypeUserPassword:
				if config.Credentials.Password != "<secret>" || config.Credentials.AWSRoleARN != "" {
					test.Errorf("password credentials = %+v, want password and no role", config.Credentials)
				}
			case auth.DatabaseAuthTypeAWSIAM:
				if config.Credentials.AWSRoleARN == "" || config.Credentials.AWSRegion != "us-east-1" || config.Credentials.Password != "" {
					test.Errorf("IAM credentials = %+v, want role and region and no password", config.Credentials)
				}
			case auth.DatabaseAuthTypeGCPIAM:
				if config.Credentials.GCPServiceAccountToImpersonate == "" || config.Credentials.GCPWorkloadIdentityPoolAudience == "" || config.Credentials.Password != "" {
					test.Errorf("GCP IAM credentials = %+v, want service account and audience and no password", config.Credentials)
				}
			case auth.DatabaseAuthTypeAzureIAM:
				if config.Credentials.AzureClientID == "" || config.Credentials.AzureTenantID == "" || config.Credentials.Password != "" {
					test.Errorf("Azure IAM credentials = %+v, want client and tenant and no password", config.Credentials)
				}
			}

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

	var bindings []Binding = specs["acmeCo/aurora/postgres-all-options"].Bindings

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

func TestSamplesFollowConnectorRules(test *testing.T) {
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
				if config.Advanced.SSLMode != "" && !slices.Contains(sslmode.PostgresModes, config.Advanced.SSLMode) {
					test.Errorf("%s: sslmode %q is not in sslmode.PostgresModes", name, config.Advanced.SSLMode)
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

	for _, authType := range auth.DatabaseAuthTypes {
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
		"timezone is a MySQL connector field": {
			key: "timezone",
			content: `
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
			key: "publication_name",
			content: `
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

	for name, document := range documents {
		test.Run(name, func(test *testing.T) {
			_, err := spectest.ParseCaptures[Spec]([]byte(document.content))

			if err == nil || !strings.Contains(err.Error(), "field "+document.key+" not found") {
				test.Fatalf("error = %v, want strict decoding to reject the %s key", err, document.key)
			}
		})
	}
}
