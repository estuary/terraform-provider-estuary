package icebergs3

import (
	"slices"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunMaterializationSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, Resource](test, "catalog_authentication", "compute/aws_access_key_id", "compute/aws_secret_access_key")
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, CatalogAuthTypes, LegacyCatalogAuthTypes, ComputeTypes, EMRAuthTypes, IdentifierCases, materialization.DBTJobTriggerModes, materialization.OnIncompatibleSchemaChanges, materialization.TargetNamingStrategies)
}

func TestSampleAuthVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		catalogAuth CatalogAuthType
		emrAuth     EMRAuthType
		ssmPrefix   string
	}{
		"acmeCo/iceberg/glue-iam-shared":           {CatalogAuthTypeIAM, EMRAuthTypeUseCatalogAuth, ""},
		"acmeCo/iceberg/glue-iam-separate-emr":     {CatalogAuthTypeIAM, EMRAuthTypeAWSAccessKey, ""},
		"acmeCo/iceberg/glue-sigv4-shared":         {CatalogAuthTypeSigV4, EMRAuthTypeUseCatalogAuth, ""},
		"acmeCo/iceberg/glue-sigv4-separate-emr":   {CatalogAuthTypeSigV4, EMRAuthTypeAWSAccessKey, ""},
		"acmeCo/iceberg/rest-oauth":                {CatalogAuthTypeClientCredentials, EMRAuthTypeAWSAccessKey, "/estuary/"},
		"acmeCo/iceberg/s3tables-iam":              {CatalogAuthTypeIAM, EMRAuthTypeUseCatalogAuth, ""},
		"acmeCo/iceberg/rest-oauth-external-token": {CatalogAuthTypeClientCredentials, EMRAuthTypeAWSAccessKey, "/estuary/"},
	}

	for name, expected := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			if !found {
				test.Fatal("materialization missing from sample")
			}

			var config Config = spec.Endpoint.Connector.Config

			if config.Credentials == nil || config.Credentials.AuthType != expected.catalogAuth {
				test.Errorf("credentials = %+v, want auth_type %q", config.Credentials, expected.catalogAuth)
			}

			if config.CatalogAuthentication != nil {
				test.Errorf("catalog_authentication = %+v, want unset", config.CatalogAuthentication)
			}

			if config.Compute.Credentials == nil || config.Compute.Credentials.AuthType != expected.emrAuth {
				test.Errorf("compute.credentials = %+v, want auth_type %q", config.Compute.Credentials, expected.emrAuth)
			}

			if config.Compute.SystemsManagerPrefix != expected.ssmPrefix {
				test.Errorf("systems_manager_prefix = %q, want %q", config.Compute.SystemsManagerPrefix, expected.ssmPrefix)
			}

			if expected.emrAuth == EMRAuthTypeUseCatalogAuth && expected.catalogAuth == CatalogAuthTypeClientCredentials {
				test.Error("sample combines UseCatalogAuth with OAuth, which the connector rejects")
			}
		})
	}
}

func TestSampleCredentialFields(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	var sigv4 Config = specs["acmeCo/iceberg/glue-sigv4-separate-emr"].Endpoint.Connector.Config

	var oauth *CatalogCredentials = specs["acmeCo/iceberg/rest-oauth-external-token"].Endpoint.Connector.Config.Credentials

	var s3Tables Config = specs["acmeCo/iceberg/s3tables-iam"].Endpoint.Connector.Config

	var glue Config = specs["acmeCo/iceberg/glue-sigv4-shared"].Endpoint.Connector.Config

	if sigv4.Credentials.AWSAccessKeyID != "AKIACATALOGXXXXXXXXX" || sigv4.Credentials.AWSRegion != "us-east-2" {
		test.Errorf("SigV4 catalog credentials = %+v", sigv4.Credentials)
	}

	if sigv4.Compute.Credentials.AWSAccessKeyID != "AKIAEMRXXXXXXXXXXXXX" {
		test.Errorf("EMR access key = %q, want the EMR-specific key", sigv4.Compute.Credentials.AWSAccessKeyID)
	}

	if oauth.OAuth2ServerURI != "https://login.dremio.cloud/oauth/token" || oauth.Credential != "<client_id>:<client_secret>" || oauth.Scope != "dremio.all" {
		test.Errorf("OAuth credentials = %+v", oauth)
	}

	if s3Tables.Warehouse != "arn:aws:s3tables:us-east-1:123456789012:bucket/my-table-bucket" || s3Tables.BaseLocation != "" {
		test.Errorf("S3 Tables warehouse = %q, base_location = %q", s3Tables.Warehouse, s3Tables.BaseLocation)
	}

	if s3Tables.Advanced == nil || s3Tables.Advanced.FieldNameCase != IdentifierCaseLowercase {
		test.Errorf("S3 Tables advanced = %+v, want field_name_case lowercase", s3Tables.Advanced)
	}

	if !glue.HardDelete || glue.Compute.BucketPath != "staging" {
		test.Errorf("hard_delete = %v, bucket_path = %q", glue.HardDelete, glue.Compute.BucketPath)
	}

	if glue.GlueOptimizers == nil || !glue.GlueOptimizers.EnableCompaction || glue.GlueOptimizers.SnapshotRetentionDays != 0 {
		test.Errorf("glue_optimizers = %+v, want compaction enabled and Glue default retention", glue.GlueOptimizers)
	}
}

func TestSampleLegacyShapes(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	var alternatives map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/alternatives.yaml")

	var config Config = specs["acmeCo/iceberg/legacy"].Endpoint.Connector.Config

	var legacy *LegacyCatalogAuthentication = config.CatalogAuthentication

	var legacyOAuth *LegacyCatalogAuthentication = alternatives["acmeCo/iceberg/legacy-oauth"].Endpoint.Connector.Config.CatalogAuthentication

	if config.Credentials != nil {
		test.Errorf("credentials = %+v, want unset for the legacy shape", config.Credentials)
	}

	if legacy == nil || legacy.CatalogAuthType != LegacyCatalogAuthTypeSigV4 || legacy.Region != "us-east-1" {
		test.Errorf("catalog_authentication = %+v, want AWS SigV4 with region us-east-1", legacy)
	}

	if config.Compute.Credentials != nil || config.Compute.AWSAccessKeyID != "AKIAXXXXXXXXXXXXXXXX" {
		test.Errorf("compute = %+v, want top-level legacy keys and no credentials", config.Compute)
	}

	if legacyOAuth == nil || legacyOAuth.CatalogAuthType != LegacyCatalogAuthTypeClientCredentials || legacyOAuth.OAuth2ServerURI != "v1/oauth/tokens" {
		test.Errorf("legacy OAuth catalog_authentication = %+v", legacyOAuth)
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/analytics/materialize-iceberg"]

	var config Config = spec.Endpoint.Connector.Config

	expectedOptimizers := GlueOptimizers{
		ExecutionRoleARN:          "arn:aws:iam::123456789012:role/glue-table-optimizer",
		EnableCompaction:          true,
		EnableRetention:           true,
		SnapshotRetentionDays:     5,
		NumberOfSnapshotsToRetain: 1,
		EnableOrphanFileDeletion:  true,
		OrphanFileRetentionDays:   3,
	}

	if config.Warehouse != "123456789012" {
		test.Errorf("warehouse = %q, want the quoted account ID kept as a string", config.Warehouse)
	}

	if config.SyncSchedule == nil || config.SyncSchedule.SyncFrequency != "30m" {
		test.Errorf("syncSchedule = %+v, want syncFrequency 30m", config.SyncSchedule)
	}

	if config.DBTJobTrigger != nil {
		test.Errorf("dbt_job_trigger = %+v, want unset", config.DBTJobTrigger)
	}

	if config.GlueOptimizers == nil || *config.GlueOptimizers != expectedOptimizers {
		test.Errorf("glue_optimizers = %+v, want %+v", config.GlueOptimizers, expectedOptimizers)
	}

	if len(spec.Bindings) != 2 {
		test.Fatalf("got %d bindings, want 2", len(spec.Bindings))
	}

	var customers Binding = spec.Bindings[1]

	if spec.Bindings[0].Fields != nil || spec.Bindings[0].Resource.Namespace != "" {
		test.Errorf("minimal binding = %+v, want no fields or namespace", spec.Bindings[0])
	}

	if customers.Resource.Namespace != "crm" || customers.Resource.AdditionalTableProperties["write.parquet.compression-codec"] != "zstd" {
		test.Errorf("customers resource = %+v", customers.Resource)
	}

	if customers.Fields == nil || customers.Fields.Recommended != materialization.FieldDepthUnlimited {
		test.Errorf("customers fields = %+v, want every field recommended", customers.Fields)
	}
}

func TestSampleSharedOptions(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/alternatives.yaml")

	var spec Spec = specs["acmeCo/iceberg/all-options"]

	var config Config = spec.Endpoint.Connector.Config

	expectedTrigger := materialization.DBTJobTrigger{
		JobID:     "<job-id>",
		AccountID: "<account-id>",
		APIKey:    "<dbt-api-key>",
		AccessURL: "https://cloud.getdbt.com",
		Cause:     "Estuary",
		Mode:      materialization.DBTJobTriggerModeReplace,
		Interval:  "1h",
	}

	expectedSchedule := materialization.SyncSchedule{
		SyncFrequency:       "30m",
		Timezone:            "UTC",
		FastSyncStartTime:   "09:00",
		FastSyncStopTime:    "17:00",
		FastSyncEnabledDays: "M-F",
	}

	expectedProperties := []SparkJobProperty{
		{
			Key:   "spark.executor.memory",
			Value: "8g",
		},
	}

	if config.DBTJobTrigger == nil || *config.DBTJobTrigger != expectedTrigger {
		test.Errorf("dbt_job_trigger = %+v, want %+v", config.DBTJobTrigger, expectedTrigger)
	}

	if config.SyncSchedule == nil || *config.SyncSchedule != expectedSchedule {
		test.Errorf("syncSchedule = %+v, want %+v", config.SyncSchedule, expectedSchedule)
	}

	if !slices.Equal(config.Compute.SparkJobProperties, expectedProperties) {
		test.Errorf("spark_job_properties = %+v, want %+v", config.Compute.SparkJobProperties, expectedProperties)
	}

	if !slices.Equal(spec.Bindings[0].Fields.Exclude, []string{"someLargeField"}) {
		test.Errorf("fields.exclude = %v, want [someLargeField]", spec.Bindings[0].Fields.Exclude)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var catalogAuthTypes *spectest.EnumSamples[CatalogAuthType] = spectest.NewEnumSamples(test, "catalog auth_type", CatalogAuthTypes)
	var emrAuthTypes *spectest.EnumSamples[EMRAuthType] = spectest.NewEnumSamples(test, "EMR auth_type", EMRAuthTypes)
	var identifierCases *spectest.EnumSamples[IdentifierCase] = spectest.NewEnumSamples(test, "identifier case", IdentifierCases)

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			if config.Compute.ComputeType != ComputeTypeEMRServerless {
				test.Errorf("%s: compute_type = %q, want %q", name, config.Compute.ComputeType, ComputeTypeEMRServerless)
			}

			if config.Credentials != nil {
				catalogAuthTypes.Add(name, config.Credentials.AuthType)
			}

			if config.Compute.Credentials != nil {
				emrAuthTypes.Add(name, config.Compute.Credentials.AuthType)
			}

			if config.Advanced != nil {
				for _, identifierCase := range []IdentifierCase{config.Advanced.TableIdentifierCase, config.Advanced.FieldNameCase} {
					if identifierCase != "" {
						identifierCases.Add(name, identifierCase)
					}
				}
			}
		}
	}

	catalogAuthTypes.AssertAllSampled()

	emrAuthTypes.AssertAllSampled()
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"delta_updates is not a merge connector field": {
			Key: "delta_updates",
			Content: `
				materializations:
					acmeCo/iceberg/delta:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-iceberg:v1
								config:
									url: https://glue.us-east-1.amazonaws.com/iceberg
									warehouse: "123456789012"
									namespace: analytics
									compute:
										compute_type: AWS EMR Serverless
						bindings:
							- resource:
									table: orders
									delta_updates: true
								source: acmeCo/orders
			`,
		},
		"misspelled compute key": {
			Key: "applicationId",
			Content: `
				materializations:
					acmeCo/iceberg/typo:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-iceberg:v1
								config:
									url: https://glue.us-east-1.amazonaws.com/iceberg
									warehouse: "123456789012"
									namespace: analytics
									compute:
										compute_type: AWS EMR Serverless
										applicationId: 00abcdefghijklmn
						bindings: []
			`,
		},
	}

	spectest.AssertUnknownMaterializationKeys[Spec](test, documents)
}
