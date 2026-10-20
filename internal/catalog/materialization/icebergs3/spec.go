package icebergs3

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// CatalogAuthType identifies how the connector authenticates against the REST catalog.
type CatalogAuthType string

// ComputeType identifies the compute backend running the Spark merge jobs.
type ComputeType string

// EMRAuthType identifies how the connector authenticates against EMR and the staging bucket.
type EMRAuthType string

// IdentifierCase controls the casing applied to created identifiers.
type IdentifierCase string

// Image is the Iceberg connector image.
const Image string = "ghcr.io/estuary/materialize-iceberg:v1"

// ConnectorName ends the catalog name of every task of the connector, as the dashboard names them.
const ConnectorName string = "materialize-iceberg"

const (
	CatalogAuthTypeIAM               CatalogAuthType = "AWS IAM"
	CatalogAuthTypeSigV4             CatalogAuthType = "AWS SigV4"
	CatalogAuthTypeClientCredentials CatalogAuthType = "OAuth 2.0 Client Credentials"
)

// CatalogAuthTypes lists every supported catalog auth type, useful for schema validators.
var CatalogAuthTypes = []CatalogAuthType{
	CatalogAuthTypeIAM,
	CatalogAuthTypeSigV4,
	CatalogAuthTypeClientCredentials,
}

const (
	ComputeTypeEMRServerless ComputeType = "AWS EMR Serverless"
)

// ComputeTypes lists every supported compute type, useful for schema validators.
var ComputeTypes = []ComputeType{
	ComputeTypeEMRServerless,
}

const (
	EMRAuthTypeUseCatalogAuth EMRAuthType = "UseCatalogAuth"
	EMRAuthTypeAWSAccessKey   EMRAuthType = "AWSAccessKey"
)

// EMRAuthTypes lists every supported EMR auth type, useful for schema validators.
var EMRAuthTypes = []EMRAuthType{
	EMRAuthTypeUseCatalogAuth,
	EMRAuthTypeAWSAccessKey,
}

const (
	IdentifierCaseLowercase IdentifierCase = "lowercase"
	IdentifierCaseUppercase IdentifierCase = "uppercase"
	IdentifierCasePreserve  IdentifierCase = "preserve"
)

// IdentifierCases lists every supported identifier case, useful for schema validators.
var IdentifierCases = []IdentifierCase{
	IdentifierCaseLowercase,
	IdentifierCaseUppercase,
	IdentifierCasePreserve,
}

// CatalogCredentials carries the catalog authentication, with the fields relevant to its AuthType.
type CatalogCredentials struct {
	AuthType           CatalogAuthType `yaml:"auth_type"`
	AWSRoleARN         string          `yaml:"aws_role_arn,omitempty"`
	AWSAccessKeyID     string          `yaml:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string          `yaml:"aws_secret_access_key,omitempty"`
	AWSRegion          string          `yaml:"aws_region,omitempty"`
	OAuth2ServerURI    string          `yaml:"oauth2_server_uri,omitempty"`
	Credential         string          `yaml:"credential,omitempty"`
	Scope              string          `yaml:"scope,omitempty"`
}

// LegacyCatalogAuthentication is the deprecated form of CatalogCredentials, used only when credentials is absent.
type LegacyCatalogAuthentication struct {
	CatalogAuthType    CatalogAuthType `yaml:"catalog_auth_type"`
	AWSAccessKeyID     string          `yaml:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string          `yaml:"aws_secret_access_key,omitempty"`
	Region             string          `yaml:"region,omitempty"`
	OAuth2ServerURI    string          `yaml:"oauth2_server_uri,omitempty"`
	Credential         string          `yaml:"credential,omitempty"`
	Scope              string          `yaml:"scope,omitempty"`
}

// EMRCredentials carries the EMR authentication, UseCatalogAuth is only valid with AWS catalog authentication.
type EMRCredentials struct {
	AuthType           EMRAuthType `yaml:"auth_type"`
	AWSAccessKeyID     string      `yaml:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string      `yaml:"aws_secret_access_key,omitempty"`
}

// SparkJobProperty overrides one of the connector's allowed Spark job properties.
type SparkJobProperty struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

// Compute carries the AWS EMR Serverless settings used to stage files and run the merge jobs.
type Compute struct {
	ComputeType          ComputeType        `yaml:"compute_type"`
	Region               string             `yaml:"region"`
	ApplicationID        string             `yaml:"application_id"`
	ExecutionRoleARN     string             `yaml:"execution_role_arn"`
	Bucket               string             `yaml:"bucket"`
	BucketPath           string             `yaml:"bucket_path,omitempty"`
	SystemsManagerPrefix string             `yaml:"systems_manager_prefix,omitempty"`
	Credentials          *EMRCredentials    `yaml:"credentials,omitempty"`
	AWSAccessKeyID       string             `yaml:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey   string             `yaml:"aws_secret_access_key,omitempty"`
	SparkJobProperties   []SparkJobProperty `yaml:"spark_job_properties,omitempty"`
}

// Advanced carries the advanced endpoint options, including identifier casing and feature flags.
type Advanced struct {
	TableIdentifierCase IdentifierCase `yaml:"table_identifier_case,omitempty"`
	FieldNameCase       IdentifierCase `yaml:"field_name_case,omitempty"`
	FeatureFlags        string         `yaml:"feature_flags,omitempty"`
}

// GlueOptimizers configures the AWS Glue managed table optimizers, only applicable with a Glue catalog.
type GlueOptimizers struct {
	ExecutionRoleARN          string `yaml:"execution_role_arn,omitempty"`
	EnableCompaction          bool   `yaml:"enable_compaction,omitempty"`
	EnableRetention           bool   `yaml:"enable_retention,omitempty"`
	SnapshotRetentionDays     int    `yaml:"snapshot_retention_days,omitempty"`
	NumberOfSnapshotsToRetain int    `yaml:"number_of_snapshots_to_retain,omitempty"`
	EnableOrphanFileDeletion  bool   `yaml:"enable_orphan_file_deletion,omitempty"`
	OrphanFileRetentionDays   int    `yaml:"orphan_file_retention_days,omitempty"`
}

// Config carries the ghcr.io/estuary/materialize-iceberg connector configurations, including
// the catalog, its authentication and the EMR Serverless compute running the merge jobs.
type Config struct {
	URL                   string                         `yaml:"url"`
	Warehouse             string                         `yaml:"warehouse"`
	Namespace             string                         `yaml:"namespace"`
	BaseLocation          string                         `yaml:"base_location,omitempty"`
	HardDelete            bool                           `yaml:"hard_delete,omitempty"`
	Credentials           *CatalogCredentials            `yaml:"credentials,omitempty"`
	CatalogAuthentication *LegacyCatalogAuthentication   `yaml:"catalog_authentication,omitempty"`
	Compute               Compute                        `yaml:"compute"`
	SyncSchedule          *materialization.SyncSchedule  `yaml:"syncSchedule,omitempty"`
	DBTJobTrigger         *materialization.DBTJobTrigger `yaml:"dbt_job_trigger,omitempty"`
	Advanced              *Advanced                      `yaml:"advanced,omitempty"`
	GlueOptimizers        *GlueOptimizers                `yaml:"glue_optimizers,omitempty"`
}

// Resource identifies the target Iceberg table, set only at creation through AdditionalTableProperties.
type Resource struct {
	Table                     string            `yaml:"table"`
	Namespace                 string            `yaml:"namespace,omitempty"`
	AdditionalTableProperties map[string]string `yaml:"additional_table_properties,omitempty"`
}

// Binding maps a source collection to the Iceberg table it materializes into.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Iceberg materialization entry, keyed by its catalog name in a catalog spec.
type Spec = materialization.Spec[EndpointSpec, Resource]
