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
	AuthType           CatalogAuthType `yaml:"auth_type" enum:"OAuth 2.0 Client Credentials,AWS SigV4,AWS IAM" description:"Authentication method: 'OAuth 2.0 Client Credentials', 'AWS SigV4' or 'AWS IAM'. AWS Glue and S3 Tables catalogs need 'AWS SigV4' or 'AWS IAM'. Only the attributes of the selected method are used."`
	AWSRoleARN         string          `yaml:"aws_role_arn,omitempty" description:"ARN of the IAM role Estuary assumes to connect, with 'AWS IAM'. See https://docs.estuary.dev/guides/iam-auth/aws/."`
	AWSAccessKeyID     string          `yaml:"aws_access_key_id,omitempty" description:"AWS access key ID of the IAM user the connector authenticates as, with 'AWS SigV4'."`
	AWSSecretAccessKey string          `yaml:"aws_secret_access_key,omitempty" description:"AWS secret access key of the IAM user, with 'AWS SigV4'. Write-only."`
	AWSRegion          string          `yaml:"aws_region,omitempty" description:"AWS region of the catalog (e.g., 'us-east-1'), with 'AWS SigV4' and 'AWS IAM'."`
	OAuth2ServerURI    string          `yaml:"oauth2_server_uri,omitempty" credentials:"forbidden" description:"OAuth 2.0 token endpoint, required with 'OAuth 2.0 Client Credentials': a path relative to url, 'v1/oauth/tokens' (the dashboard default) for most catalogs, or a full http(s) URL when the token endpoint is on another host (e.g., 'https://login.dremio.cloud/oauth/token'). The plan rejects credentials in it, such as user:password@ or a password or token option, as state and logs would hold them in plain text. Credentials found in the live value show as ***."`
	Credential         string          `yaml:"credential,omitempty" description:"Catalog credential, with 'OAuth 2.0 Client Credentials': '<client_id>:<client_secret>' for OAuth client credentials, or '<token>' for Bearer authentication. Write-only."`
	Scope              string          `yaml:"scope,omitempty" description:"Authorization scope requested with 'OAuth 2.0 Client Credentials' (e.g., 'PRINCIPAL_ROLE:<role>')."`
}

// LegacyCatalogAuthentication is the deprecated CatalogCredentials, used only without credentials.
type LegacyCatalogAuthentication struct {
	CatalogAuthType    LegacyCatalogAuthType `yaml:"catalog_auth_type" enum:"OAuth 2.0 Client Credentials,AWS SigV4" description:"Authentication method: 'OAuth 2.0 Client Credentials' or 'AWS SigV4'. Only the attributes of the selected method are used."`
	AWSAccessKeyID     string                `yaml:"aws_access_key_id,omitempty" description:"AWS access key ID of the IAM user the connector authenticates as, with 'AWS SigV4'."`
	AWSSecretAccessKey string                `yaml:"aws_secret_access_key,omitempty" description:"AWS secret access key of the IAM user, with 'AWS SigV4'. Write-only."`
	Region             string                `yaml:"region,omitempty" description:"AWS region of the catalog (e.g., 'us-east-1'), with 'AWS SigV4'."`
	OAuth2ServerURI    string                `yaml:"oauth2_server_uri,omitempty" credentials:"forbidden" description:"OAuth 2.0 token endpoint, required with 'OAuth 2.0 Client Credentials': a path relative to url, 'v1/oauth/tokens' for most catalogs, or a full http(s) URL. The plan rejects credentials in it, such as user:password@ or a password or token option, as state and logs would hold them in plain text. Credentials found in the live value show as ***."`
	Credential         string                `yaml:"credential,omitempty" description:"Catalog credential, with 'OAuth 2.0 Client Credentials': '<client_id>:<client_secret>', or '<token>' for Bearer authentication. Write-only."`
	Scope              string                `yaml:"scope,omitempty" description:"Authorization scope requested with 'OAuth 2.0 Client Credentials'."`
	SigningName        string                `yaml:"signing_name,omitempty" description:"Ignored by the connector, which derives the SigV4 signing name from url. Kept so existing materializations still decode."`
}

// EMRCredentials is the EMR authentication; UseCatalogAuth requires AWS catalog authentication.
type EMRCredentials struct {
	AuthType           EMRAuthType `yaml:"auth_type" enum:"AWSAccessKey,UseCatalogAuth" description:"Authentication method: 'AWSAccessKey', or 'UseCatalogAuth' to reuse the catalog credentials, which then must use 'AWS SigV4' or 'AWS IAM'. Only the attributes of the selected method are used."`
	AWSAccessKeyID     string      `yaml:"aws_access_key_id,omitempty" description:"AWS access key ID of the IAM user that runs the EMR jobs and writes to the staging bucket, with 'AWSAccessKey'."`
	AWSSecretAccessKey string      `yaml:"aws_secret_access_key,omitempty" description:"AWS secret access key of the IAM user, with 'AWSAccessKey'. Write-only."`
}

// SparkJobProperty overrides an allowed Spark job property.
type SparkJobProperty struct {
	Key   string `yaml:"key" enum:"spark.executor.instances,spark.dynamicAllocation.initialExecutors,spark.dynamicAllocation.maxExecutors" description:"Spark property to override: 'spark.executor.instances', 'spark.dynamicAllocation.initialExecutors' or 'spark.dynamicAllocation.maxExecutors'."`
	Value string `yaml:"value" description:"Value of the property, letters and digits only (e.g., '8')."`
}

// Compute holds the EMR Serverless settings that stage files and run the merge jobs.
type Compute struct {
	ComputeType          ComputeType        `yaml:"compute_type" enum:"AWS EMR Serverless" description:"Compute backend running the merge jobs. Only 'AWS EMR Serverless' is supported."`
	Region               string             `yaml:"region" description:"AWS region of the EMR Serverless application and the staging bucket (e.g., 'us-east-1')."`
	ApplicationID        string             `yaml:"application_id" description:"ID of the EMR Serverless application, which needs the Spark runtime."`
	ExecutionRoleARN     string             `yaml:"execution_role_arn" description:"ARN of the IAM role the EMR Serverless jobs run as."`
	Bucket               string             `yaml:"bucket" description:"S3 bucket where data files are staged before they're merged into tables."`
	BucketPath           string             `yaml:"bucket_path,omitempty" description:"Prefix of the staged data files in bucket."`
	SystemsManagerPrefix string             `yaml:"systems_manager_prefix,omitempty" description:"AWS Systems Manager path, starting and ending with '/' (e.g., '/estuary/'), where the connector stores the catalog credential for the EMR jobs. Required with 'OAuth 2.0 Client Credentials' catalog authentication."`
	Credentials          *EMRCredentials    `yaml:"credentials,omitempty" description:"How the connector authenticates with EMR and the staging bucket. Set it, or the deprecated aws_access_key_id and aws_secret_access_key, for the materialization to connect."`
	AWSAccessKeyID       string             `yaml:"aws_access_key_id,omitempty" description:"Deprecated form of credentials with auth_type 'AWSAccessKey', still accepted for existing materializations."`
	AWSSecretAccessKey   string             `yaml:"aws_secret_access_key,omitempty" description:"Deprecated form of credentials with auth_type 'AWSAccessKey', still accepted for existing materializations. Write-only."`
	SparkJobProperties   []SparkJobProperty `yaml:"spark_job_properties,omitempty" description:"Spark properties overriding the EMR Serverless defaults of the merge jobs."`
}

// Advanced holds advanced endpoint options; LowercaseColumnNames is the deprecated form of FieldNameCase lowercase.
type Advanced struct {
	LowercaseColumnNames bool           `yaml:"lowercase_column_names,omitempty" description:"Deprecated form of field_name_case 'lowercase', still accepted for existing materializations. It conflicts with field_name_case 'uppercase' or 'preserve'."`
	TableIdentifierCase  IdentifierCase `yaml:"table_identifier_case,omitempty" enum:"lowercase,uppercase,preserve" description:"Casing of the namespace and table names of created tables: 'lowercase' (default), 'uppercase', for catalogs resolving unquoted identifiers as upper case such as Snowflake's, or 'preserve'. Existing tables are never renamed."`
	FieldNameCase        IdentifierCase `yaml:"field_name_case,omitempty" enum:"lowercase,uppercase,preserve" description:"Casing of the names of created columns: 'preserve' (default), 'lowercase', needed to query S3 Table Buckets with Athena, or 'uppercase'. Existing columns are matched case-insensitively and never renamed."`
	FeatureFlags         string         `yaml:"feature_flags,omitempty" description:"Connector settings for edge cases and experimental features, see https://docs.estuary.dev/guides/advanced-usage/feature-flags/. Intended for Estuary internal use: change them only as directed by Estuary support."`
}

// GlueOptimizers configures AWS Glue table optimizers, only with a Glue catalog.
type GlueOptimizers struct {
	ExecutionRoleARN          string `yaml:"execution_role_arn,omitempty" description:"ARN of the IAM role Glue assumes to optimize tables. It needs read and write access to the table data in S3 and access to the Glue catalog. Required when any optimizer is enabled."`
	EnableCompaction          bool   `yaml:"enable_compaction,omitempty" description:"Compacts small data files to speed up queries. Defaults to false."`
	EnableRetention           bool   `yaml:"enable_retention,omitempty" description:"Removes old table snapshots to cut storage costs. Defaults to false."`
	SnapshotRetentionDays     int    `yaml:"snapshot_retention_days,omitempty" description:"Days snapshots are kept, with enable_retention. Defaults to Glue's 5."`
	NumberOfSnapshotsToRetain int    `yaml:"number_of_snapshots_to_retain,omitempty" description:"Snapshots always kept, whatever their age, with enable_retention. Defaults to Glue's 1."`
	EnableOrphanFileDeletion  bool   `yaml:"enable_orphan_file_deletion,omitempty" description:"Deletes files no table snapshot references. Defaults to false."`
	OrphanFileRetentionDays   int    `yaml:"orphan_file_retention_days,omitempty" description:"Days orphan files are kept before deletion, with enable_orphan_file_deletion. Defaults to Glue's 3."`
}

// Config is the materialize-iceberg connector configuration.
type Config struct {
	URL                   string                         `yaml:"url" credentials:"forbidden" description:"Base URL of the Iceberg REST catalog: 'https://glue.<region>.amazonaws.com/iceberg' for AWS Glue, 'https://s3tables.<region>.amazonaws.com/iceberg' for S3 Tables, or the URL of another REST catalog. The plan rejects credentials in it, such as user:password@ or a password or token option, as state and logs would hold them in plain text. Credentials found in the live value show as ***."`
	Warehouse             string                         `yaml:"warehouse" description:"Warehouse to connect to: the AWS account ID without hyphens for AWS Glue, or the table bucket ARN for S3 Tables."`
	Namespace             string                         `yaml:"namespace" x-default-namespace:"true" description:"Namespace of the materialized tables, unless a binding sets its own. It can't contain dots."`
	BaseLocation          string                         `yaml:"base_location,omitempty" description:"S3 location of the table data and metadata (e.g., 's3://<bucket>/<prefix>/'). Required with AWS Glue, optional with other catalogs."`
	HardDelete            bool                           `yaml:"hard_delete,omitempty" description:"Deletes from the table the documents deleted in the source. Otherwise, the _meta/op column marks deleted rows (soft delete). Defaults to false."`
	Credentials           *CatalogCredentials            `yaml:"credentials,omitempty" description:"How the connector authenticates with the catalog. Set it, or the deprecated catalog_authentication, for the materialization to connect."`
	CatalogAuthentication *LegacyCatalogAuthentication   `yaml:"catalog_authentication,omitempty" description:"Deprecated form of credentials, still accepted for existing materializations. Ignored when credentials is set."`
	Compute               Compute                        `yaml:"compute" description:"EMR Serverless application that merges the staged data into the tables."`
	SyncSchedule          *materialization.SyncSchedule  `yaml:"syncSchedule,omitempty" description:"When and how often transactions run."`
	DBTJobTrigger         *materialization.DBTJobTrigger `yaml:"dbt_job_trigger,omitempty" description:"Triggers a dbt Cloud job when new data is materialized."`
	Advanced              *Advanced                      `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
	GlueOptimizers        *GlueOptimizers                `yaml:"glue_optimizers,omitempty" description:"AWS Glue managed table optimizers, registered on each table. Only with AWS Glue as the catalog."`
}

// Resource identifies the target Iceberg table; AdditionalTableProperties apply only at creation.
type Resource struct {
	Table                     string            `yaml:"table" x-collection-name:"true" description:"Iceberg table to materialize into. The connector creates it when missing."`
	Namespace                 string            `yaml:"namespace,omitempty" x-schema-name:"true" description:"Namespace of the table, overriding config.namespace."`
	AdditionalTableProperties map[string]string `yaml:"additional_table_properties,omitempty" description:"Iceberg table properties set when the table is created (e.g., 'write.parquet.compression-codec' = 'zstd'). They can't be changed afterwards."`
}

// Binding maps a source collection to its Iceberg table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is an Iceberg materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
