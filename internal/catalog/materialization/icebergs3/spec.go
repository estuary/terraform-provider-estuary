package icebergs3

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// CatalogAuthType is how the connector authenticates against the REST catalog.
type CatalogAuthType string

// ComputeType is the compute backend running the Spark merge jobs.
type ComputeType string

// EMRAuthType is how the connector authenticates against EMR and the staging bucket.
type EMRAuthType string

// IdentifierCase is the casing of created identifiers.
type IdentifierCase string

// LegacyCatalogAuthType is the deprecated REST catalog authentication type.
type LegacyCatalogAuthType string

// Image is the Iceberg connector image.
const Image string = "ghcr.io/estuary/materialize-iceberg:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
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
	LegacyCatalogAuthTypeSigV4             LegacyCatalogAuthType = "AWS SigV4"
	LegacyCatalogAuthTypeClientCredentials LegacyCatalogAuthType = "OAuth 2.0 Client Credentials"
)

// LegacyCatalogAuthTypes lists every deprecated catalog auth type, useful for schema validators.
var LegacyCatalogAuthTypes = []LegacyCatalogAuthType{
	LegacyCatalogAuthTypeSigV4,
	LegacyCatalogAuthTypeClientCredentials,
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

// CatalogCredentials is the catalog authentication, using the fields of its AuthType.
type CatalogCredentials struct {
	AuthType           CatalogAuthType `yaml:"auth_type" enum:"OAuth 2.0 Client Credentials,AWS SigV4,AWS IAM"`
	AWSRoleARN         string          `yaml:"aws_role_arn,omitempty"`
	AWSAccessKeyID     string          `yaml:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string          `yaml:"aws_secret_access_key,omitempty"`
	AWSRegion          string          `yaml:"aws_region,omitempty"`
	OAuth2ServerURI    string          `yaml:"oauth2_server_uri,omitempty"`
	Credential         string          `yaml:"credential,omitempty"`
	Scope              string          `yaml:"scope,omitempty"`
}

// LegacyCatalogAuthentication is the deprecated CatalogCredentials, used only without credentials.
type LegacyCatalogAuthentication struct {
	CatalogAuthType    LegacyCatalogAuthType `yaml:"catalog_auth_type" enum:"OAuth 2.0 Client Credentials,AWS SigV4"`
	AWSAccessKeyID     string                `yaml:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string                `yaml:"aws_secret_access_key,omitempty"`
	Region             string                `yaml:"region,omitempty"`
	OAuth2ServerURI    string                `yaml:"oauth2_server_uri,omitempty"`
	Credential         string                `yaml:"credential,omitempty"`
	Scope              string                `yaml:"scope,omitempty"`
}

// EMRCredentials is the EMR authentication; UseCatalogAuth requires AWS catalog authentication.
type EMRCredentials struct {
	AuthType           EMRAuthType `yaml:"auth_type" enum:"AWSAccessKey,UseCatalogAuth"`
	AWSAccessKeyID     string      `yaml:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string      `yaml:"aws_secret_access_key,omitempty"`
}

// SparkJobProperty overrides an allowed Spark job property.
type SparkJobProperty struct {
	Key   string `yaml:"key" enum:"spark.executor.instances,spark.dynamicAllocation.initialExecutors,spark.dynamicAllocation.maxExecutors"`
	Value string `yaml:"value"`
}

// Compute holds the EMR Serverless settings that stage files and run the merge jobs.
type Compute struct {
	ComputeType          ComputeType        `yaml:"compute_type" enum:"AWS EMR Serverless"`
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

// Advanced holds advanced endpoint options, such as identifier casing.
type Advanced struct {
	TableIdentifierCase IdentifierCase `yaml:"table_identifier_case,omitempty" enum:"lowercase,uppercase,preserve"`
	FieldNameCase       IdentifierCase `yaml:"field_name_case,omitempty" enum:"lowercase,uppercase,preserve"`
	FeatureFlags        string         `yaml:"feature_flags,omitempty"`
}

// GlueOptimizers configures AWS Glue table optimizers, only with a Glue catalog.
type GlueOptimizers struct {
	ExecutionRoleARN          string `yaml:"execution_role_arn,omitempty"`
	EnableCompaction          bool   `yaml:"enable_compaction,omitempty"`
	EnableRetention           bool   `yaml:"enable_retention,omitempty"`
	SnapshotRetentionDays     int    `yaml:"snapshot_retention_days,omitempty"`
	NumberOfSnapshotsToRetain int    `yaml:"number_of_snapshots_to_retain,omitempty"`
	EnableOrphanFileDeletion  bool   `yaml:"enable_orphan_file_deletion,omitempty"`
	OrphanFileRetentionDays   int    `yaml:"orphan_file_retention_days,omitempty"`
}

// Config is the materialize-iceberg connector configuration.
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

// Resource identifies the target Iceberg table; AdditionalTableProperties apply only at creation.
type Resource struct {
	Table                     string            `yaml:"table"`
	Namespace                 string            `yaml:"namespace,omitempty"`
	AdditionalTableProperties map[string]string `yaml:"additional_table_properties,omitempty"`
}

// Binding maps a source collection to its Iceberg table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is an Iceberg materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
