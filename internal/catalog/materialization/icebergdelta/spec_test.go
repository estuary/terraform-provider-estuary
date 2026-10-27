package icebergdelta

import (
	"slices"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
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
	spectest.AssertEnumTags[Spec](test, CatalogTypes, auth.AWSAuthTypes, materialization.OnIncompatibleSchemaChanges, materialization.TargetNamingStrategies)
}

func TestSampleAuthVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		authType    auth.AWSAuthType
		catalogType CatalogType
		glueID      string
	}{
		"acmeCo/iceberg/rest-access-key":        {auth.AWSAuthTypeAccessKey, CatalogTypeREST, ""},
		"acmeCo/iceberg/rest-iam":               {auth.AWSAuthTypeIAM, CatalogTypeREST, ""},
		"acmeCo/iceberg/glue-access-key":        {auth.AWSAuthTypeAccessKey, CatalogTypeGlue, ""},
		"acmeCo/iceberg/glue-iam":               {auth.AWSAuthTypeIAM, CatalogTypeGlue, ""},
		"acmeCo/iceberg/glue-iam-cross-account": {auth.AWSAuthTypeIAM, CatalogTypeGlue, "210987654321"},
	}

	for name, expected := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			if !found {
				test.Fatal("materialization missing from sample")
			}

			var config Config = spec.Endpoint.Connector.Config

			if config.Credentials == nil || config.Credentials.AuthType != expected.authType {
				test.Fatalf("credentials = %+v, want auth_type %q", config.Credentials, expected.authType)
			}

			switch expected.authType {
			case auth.AWSAuthTypeAccessKey:
				if config.Credentials.AWSAccessKeyID == "" || config.Credentials.AWSRoleARN != "" {
					test.Errorf("access key credentials = %+v, want awsAccessKeyId and no role", config.Credentials)
				}
			case auth.AWSAuthTypeIAM:
				if config.Credentials.AWSRoleARN == "" || config.Credentials.AWSRegion == "" || config.Credentials.AWSAccessKeyID != "" {
					test.Errorf("IAM credentials = %+v, want role and region and no keys", config.Credentials)
				}
			}

			if config.AWSAccessKeyID != "" {
				test.Errorf("legacy aws_access_key_id = %q, want unset", config.AWSAccessKeyID)
			}

			if config.Catalog.CatalogType != expected.catalogType {
				test.Errorf("catalog_type = %q, want %q", config.Catalog.CatalogType, expected.catalogType)
			}

			if config.Catalog.GlueID != expected.glueID {
				test.Errorf("glue_id = %q, want %q", config.Catalog.GlueID, expected.glueID)
			}

			if expected.catalogType == CatalogTypeREST && (config.Catalog.URI == "" || config.Catalog.Warehouse == "") {
				test.Errorf("REST catalog = %+v, want uri and warehouse", config.Catalog)
			}

			if config.UploadInterval != "PT5M" {
				test.Errorf("upload_interval = %q, want PT5M", config.UploadInterval)
			}
		})
	}
}

func TestSampleLegacyKeys(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	var config Config = specs["acmeCo/iceberg/legacy-keys"].Endpoint.Connector.Config

	if config.Credentials != nil {
		test.Errorf("credentials = %+v, want unset for the legacy shape", config.Credentials)
	}

	if config.AWSAccessKeyID != "AKIAXXXXXXXXXXXXXXXX" || config.AWSSecretAccessKey != "<secret-access-key>" {
		test.Errorf("legacy keys = %q / %q", config.AWSAccessKeyID, config.AWSSecretAccessKey)
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/analytics/materialize-s3-iceberg"]

	var config Config = spec.Endpoint.Connector.Config

	expectedCatalog := CatalogConfig{
		CatalogType: CatalogTypeREST,
		URI:         "https://catalog.example.com/api/catalog",
		Warehouse:   "my_warehouse",
		Credential:  "<client_id>:<client_secret>",
		Token:       "<bearer-token>",
		Scope:       "PRINCIPAL_ROLE:ALL",
	}

	if config.Catalog != expectedCatalog {
		test.Errorf("catalog = %+v, want %+v", config.Catalog, expectedCatalog)
	}

	if config.Advanced == nil || config.Advanced.NanosecondTimestamps || config.Advanced.VariantColumns {
		test.Errorf("advanced = %+v, want both format v3 options disabled", config.Advanced)
	}

	if len(spec.Bindings) != 2 {
		test.Fatalf("got %d bindings, want 2", len(spec.Bindings))
	}

	var customers Binding = spec.Bindings[1]

	if customers.Resource.Namespace != "crm" || customers.Resource.AdditionalTableProperties["write.parquet.compression-codec"] != "zstd" {
		test.Errorf("customers resource = %+v", customers.Resource)
	}

	if customers.Fields == nil || customers.Fields.Recommended != materialization.FieldDepthUnlimited {
		test.Errorf("customers fields = %+v, want every field recommended", customers.Fields)
	}
}

func TestSampleAlternatives(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/alternatives.yaml")

	var spec Spec = specs["acmeCo/iceberg/s3-compatible-v3"]

	var config Config = spec.Endpoint.Connector.Config

	expectedAdvanced := Advanced{
		NanosecondTimestamps: true,
		VariantColumns:       true,
		FeatureFlags:         "retain_existing_data_on_backfill,nested_dot_hash_location_style",
	}

	if config.S3Endpoint != "https://minio.example.com" {
		test.Errorf("s3_endpoint = %q", config.S3Endpoint)
	}

	if config.Advanced == nil || *config.Advanced != expectedAdvanced {
		test.Errorf("advanced = %+v, want %+v", config.Advanced, expectedAdvanced)
	}

	if version := spec.Bindings[0].Resource.AdditionalTableProperties["format-version"]; version != "3" {
		test.Errorf("format-version = %q, want 3", version)
	}

	if !slices.Equal(spec.Bindings[0].Fields.Exclude, []string{"someLargeField"}) {
		test.Errorf("fields.exclude = %v, want [someLargeField]", spec.Bindings[0].Fields.Exclude)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var authTypes *spectest.EnumSamples[auth.AWSAuthType] = spectest.NewEnumSamples(test, "auth_type", auth.AWSAuthTypes)
	var catalogTypes *spectest.EnumSamples[CatalogType] = spectest.NewEnumSamples(test, "catalog_type", CatalogTypes)

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			if config.Credentials != nil {
				authTypes.Add(name, config.Credentials.AuthType)
			}

			catalogTypes.Add(name, config.Catalog.CatalogType)
		}
	}

	authTypes.AssertAllSampled()

	catalogTypes.AssertAllSampled()
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"merge connector compute block": {
			Key: "compute",
			Content: `
				materializations:
					acmeCo/iceberg/merge:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-s3-iceberg:v2
								config:
									bucket: my-iceberg-bucket
									region: us-east-1
									namespace: analytics
									catalog:
										catalog_type: AWS Glue
									compute:
										compute_type: AWS EMR Serverless
						bindings: []
			`,
		},
		"snake case access key under credentials": {
			Key: "aws_access_key_id",
			Content: `
				materializations:
					acmeCo/iceberg/typo:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-s3-iceberg:v2
								config:
									credentials:
										auth_type: AWSAccessKey
										aws_access_key_id: AKIAXXXXXXXXXXXXXXXX
									bucket: my-iceberg-bucket
									region: us-east-1
									namespace: analytics
									catalog:
										catalog_type: AWS Glue
						bindings: []
			`,
		},
	}

	spectest.AssertUnknownMaterializationKeys[Spec](test, documents)
}
