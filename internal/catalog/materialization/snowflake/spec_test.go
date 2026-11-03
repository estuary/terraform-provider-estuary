package snowflake

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunMaterializationSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, Resource](test)
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, AuthTypes, TimestampTypes, materialization.DBTJobTriggerModes, materialization.OnIncompatibleSchemaChanges, materialization.TargetNamingStrategies)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		authType      AuthType
		timestampType TimestampType
		hardDelete    bool
	}{
		"acmeCo/snowflake/key-pair":      {AuthTypeJWT, TimestampTypeLTZ, false},
		"acmeCo/snowflake/user-password": {AuthTypeUserPassword, TimestampTypeNTZDiscard, false},
		"acmeCo/snowflake/delta-updates": {AuthTypeJWT, TimestampTypeNTZNormalize, false},
		"acmeCo/snowflake/timestamp-tz":  {AuthTypeJWT, TimestampTypeTZ, true},
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

			if config.Credentials.AuthType != expected.authType || config.Credentials.User != "snowflake_user" {
				test.Errorf("credentials = %+v, want auth_type %q for snowflake_user", config.Credentials, expected.authType)
			}

			if config.TimestampType != expected.timestampType {
				test.Errorf("timestamp_type = %q, want %q", config.TimestampType, expected.timestampType)
			}

			if config.HardDelete != expected.hardDelete {
				test.Errorf("hardDelete = %v, want %v", config.HardDelete, expected.hardDelete)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var config Config = specs["acmeCo/analytics/materialize-snowflake"].Endpoint.Connector.Config

	expectedSyncSchedule := materialization.SyncSchedule{
		SyncFrequency:       "1h",
		Timezone:            "America/New_York",
		FastSyncStartTime:   "09:00",
		FastSyncStopTime:    "17:00",
		FastSyncEnabledDays: "M-F",
	}

	expectedDBTJobTrigger := materialization.DBTJobTrigger{
		JobID:     "12345",
		AccountID: "67890",
		APIKey:    "<dbt-api-key>",
		AccessURL: "https://cloud.getdbt.com",
		Cause:     "Estuary",
		Mode:      materialization.DBTJobTriggerModeReplace,
		Interval:  "30m",
	}

	expectedAdvanced := Advanced{
		DisableFieldTruncation: true,
		NoFlowDocument:         true,
		FeatureFlags:           "snowpipe_streaming_v2",
	}

	if config.Warehouse != "acmeCo_warehouse" || config.Role != "acmeCo_role" || config.Account != "acmeCo_account" {
		test.Errorf("warehouse = %q, role = %q, account = %q", config.Warehouse, config.Role, config.Account)
	}

	if config.SyncSchedule == nil || *config.SyncSchedule != expectedSyncSchedule {
		test.Errorf("syncSchedule = %+v, want %+v", config.SyncSchedule, expectedSyncSchedule)
	}

	if config.DBTJobTrigger == nil || *config.DBTJobTrigger != expectedDBTJobTrigger {
		test.Errorf("dbt_job_trigger = %+v, want %+v", config.DBTJobTrigger, expectedDBTJobTrigger)
	}

	if config.Advanced == nil || *config.Advanced != expectedAdvanced {
		test.Errorf("advanced = %+v, want %+v", config.Advanced, expectedAdvanced)
	}

	if !strings.HasPrefix(config.Credentials.PrivateKey, "-----BEGIN PRIVATE KEY-----\n") {
		test.Errorf("private_key = %q, want a multiline PEM key", config.Credentials.PrivateKey)
	}
}

func TestSampleBindings(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var bindings []Binding = specs["acmeCo/analytics/materialize-snowflake"].Bindings

	spectest.AssertMaterializationResources(test, bindings, []Resource{
		{Table: "orders"},
		{Table: "customers", Schema: "crm", Clustering: "region,created_at"},
		{Table: "events", DeltaUpdates: true},
	})

	if bindings[2].Fields == nil || !slices.Equal(bindings[2].Fields.Exclude, []string{"payload"}) {
		test.Errorf("events fields = %+v, want exclude [payload]", bindings[2].Fields)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var host *regexp.Regexp = regexp.MustCompile(`^[^/:]+.snowflakecomputing.com$`)
	var authTypes *spectest.EnumSamples[AuthType] = spectest.NewEnumSamples(test, "auth_type", AuthTypes)
	var timestampTypes *spectest.EnumSamples[TimestampType] = spectest.NewEnumSamples(test, "timestamp_type", TimestampTypes)
	var dbtModes *spectest.EnumSamples[materialization.DBTJobTriggerMode] = spectest.NewEnumSamples(test, "dbt_job_trigger mode", materialization.DBTJobTriggerModes)

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			var credentials Credentials = config.Credentials

			if !host.MatchString(config.Host) {
				test.Errorf("%s: host %q must be an account URL ending in .snowflakecomputing.com", name, config.Host)
			}

			authTypes.Add(name, credentials.AuthType)

			if credentials.AuthType == AuthTypeJWT && (credentials.User == "" || credentials.PrivateKey == "" || credentials.Password != "") {
				test.Errorf("%s: jwt credentials need user and private_key and no password", name)
			}

			if credentials.AuthType == AuthTypeUserPassword && (credentials.User == "" || credentials.Password == "" || credentials.PrivateKey != "") {
				test.Errorf("%s: user_password credentials need user and password and no private_key", name)
			}

			timestampTypes.Add(name, config.TimestampType)

			if config.DBTJobTrigger != nil && config.DBTJobTrigger.Mode != "" {
				dbtModes.Add(name, config.DBTJobTrigger.Mode)
			}
		}
	}

	authTypes.AssertAllSampled()

	timestampTypes.AssertAllSampled()
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"camel case private key from the docs": {
			Key: "privateKey",
			Content: `
				materializations:
					acmeCo/snowflake/typo:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-snowflake:v4
								config:
									host: orgname-accountname.snowflakecomputing.com
									database: acmeCo_db
									schema: acmeCo_flow_schema
									timestamp_type: TIMESTAMP_LTZ
									credentials:
										auth_type: jwt
										user: snowflake_user
										privateKey: <private-key>
						bindings: []
			`,
		},
		"namespace is an Iceberg resource field": {
			Key: "namespace",
			Content: `
				materializations:
					acmeCo/snowflake/namespace:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-snowflake:v4
								config:
									host: orgname-accountname.snowflakecomputing.com
									database: acmeCo_db
									schema: acmeCo_flow_schema
									timestamp_type: TIMESTAMP_LTZ
									credentials:
										auth_type: jwt
										user: snowflake_user
										private_key: <private-key>
						bindings:
							- resource:
									table: orders
									namespace: analytics
								source: acmeCo/orders
			`,
		},
	}

	spectest.AssertUnknownMaterializationKeys[Spec](test, documents)
}
