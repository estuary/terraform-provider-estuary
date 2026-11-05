package s3iceberg

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// CatalogType is the Iceberg catalog implementation.
type CatalogType string

// Image is the Iceberg delta updates connector image.
const Image string = "ghcr.io/estuary/materialize-s3-iceberg:v2"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "materialize-s3-iceberg"

const (
	CatalogTypeGlue CatalogType = "AWS Glue"
	CatalogTypeREST CatalogType = "Iceberg REST Server"
)

// CatalogTypes lists every supported catalog type, useful for schema validators.
var CatalogTypes = []CatalogType{
	CatalogTypeGlue,
	CatalogTypeREST,
}

// Credentials is the AWS authentication, using the fields of its AuthType.
type Credentials struct {
	AuthType           auth.AWSAuthType `yaml:"auth_type" enum:"AWSAccessKey,AWSIAM" description:"Authentication method: 'AWSAccessKey' or 'AWSIAM'. Only the attributes of the selected method are used."`
	AWSAccessKeyID     string           `yaml:"awsAccessKeyId,omitempty" description:"AWS access key ID of the IAM user the connector authenticates as, with 'AWSAccessKey'."`
	AWSSecretAccessKey string           `yaml:"awsSecretAccessKey,omitempty" description:"AWS secret access key of the IAM user, with 'AWSAccessKey'. Write-only."`
	AWSRoleARN         string           `yaml:"aws_role_arn,omitempty" description:"ARN of the IAM role Estuary assumes to connect, with 'AWSIAM'. See https://docs.estuary.dev/guides/iam-auth/aws/."`
	AWSRegion          string           `yaml:"aws_region,omitempty" description:"AWS region to authenticate in (e.g., 'us-east-1'), with 'AWSIAM'."`
}

// CatalogConfig holds the Iceberg catalog settings, using the fields of its CatalogType.
type CatalogConfig struct {
	CatalogType CatalogType `yaml:"catalog_type" enum:"AWS Glue,Iceberg REST Server" description:"Catalog the tables are registered in: 'Iceberg REST Server' or 'AWS Glue'. Only the attributes of the selected catalog are used."`
	GlueID      string      `yaml:"glue_id,omitempty" description:"ID of the Glue catalog, with 'AWS Glue', for cross-account access. Defaults to the account ID of the credentials."`
	URI         string      `yaml:"uri,omitempty" credentials:"forbidden" description:"URL of the REST catalog (e.g., 'https://<server>/api/catalog'). Required with 'Iceberg REST Server'. The plan rejects credentials in it, such as user:password@ or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as ***."`
	Warehouse   string      `yaml:"warehouse,omitempty" description:"Warehouse to connect to in the REST catalog. Required with 'Iceberg REST Server'."`
	Credential  string      `yaml:"credential,omitempty" description:"Credential for the REST catalog, such as '<client_id>:<client_secret>', with 'Iceberg REST Server'. Write-only."`
	Token       string      `yaml:"token,omitempty" description:"Bearer token for the REST catalog, with 'Iceberg REST Server'. Write-only."`
	Scope       string      `yaml:"scope,omitempty" description:"Scope of the requested security token, with 'Iceberg REST Server' (e.g., 'PRINCIPAL_ROLE:ALL')."`
}

// Advanced holds advanced endpoint options, such as Iceberg format v3 types.
type Advanced struct {
	NanosecondTimestamps bool   `yaml:"nanosecond_timestamps,omitempty" description:"Stores date-time columns with nanosecond precision (Iceberg format v3) instead of microseconds (v2). Changing it applies to new data only: existing rows read as null in converted columns unless the binding is backfilled. Defaults to false."`
	VariantColumns       bool   `yaml:"variant_columns,omitempty" description:"Stores object, array and multi-type fields and the root document as Iceberg variant columns (format v3) instead of JSON strings. Changing it applies to new data only: existing rows read as null in converted columns unless the binding is backfilled. Defaults to false."`
	FeatureFlags         string `yaml:"feature_flags,omitempty" description:"Connector settings for edge cases and experimental features, see https://docs.estuary.dev/guides/advanced-usage/feature-flags/. Intended for Estuary internal use: change them only as directed by Estuary support."`
}

// Config is the materialize-s3-iceberg connector configuration.
// AWSAccessKeyID and AWSSecretAccessKey are the deprecated form of Credentials.
type Config struct {
	Credentials        *Credentials  `yaml:"credentials,omitempty" description:"How the connector authenticates with AWS. Set it, or the deprecated aws_access_key_id and aws_secret_access_key, for the materialization to connect."`
	AWSAccessKeyID     string        `yaml:"aws_access_key_id,omitempty" description:"Deprecated form of credentials with auth_type 'AWSAccessKey', still accepted for existing materializations. Ignored when credentials is set."`
	AWSSecretAccessKey string        `yaml:"aws_secret_access_key,omitempty" description:"Deprecated form of credentials with auth_type 'AWSAccessKey', still accepted for existing materializations. Ignored when credentials is set. Write-only."`
	Bucket             string        `yaml:"bucket" description:"S3 bucket the data files are written to."`
	Prefix             string        `yaml:"prefix,omitempty" description:"Prefix of the data files in bucket."`
	Region             string        `yaml:"region" description:"AWS region of bucket (e.g., 'us-east-1')."`
	Namespace          string        `yaml:"namespace" description:"Namespace of the materialized tables, unless a binding sets its own. It can't contain dots."`
	UploadInterval     string        `yaml:"upload_interval,omitempty" description:"How often data files are uploaded and appended to the tables, as an ISO 8601 duration of at most 4 hours (e.g., 'PT5M', the dashboard default). Set it: the connector's validation fails without a value."`
	S3Endpoint         string        `yaml:"s3_endpoint,omitempty" credentials:"forbidden" description:"S3 endpoint URL for S3-compatible storage, which is then addressed path-style. Defaults to the AWS S3 endpoint of region. The plan rejects credentials in it, such as user:password@ or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as ***."`
	Catalog            CatalogConfig `yaml:"catalog" description:"Iceberg catalog the tables are registered in."`
	Advanced           *Advanced     `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
}

// Resource identifies the target Iceberg table; DeltaUpdates defaults to true and the connector rejects false.
type Resource struct {
	Table                     string            `yaml:"table" x-collection-name:"true" description:"Iceberg table to materialize into, created when missing. The name is lowercased and can't contain dots."`
	Namespace                 string            `yaml:"namespace,omitempty" description:"Namespace of the table, overriding config.namespace. It's lowercased and can't contain dots."`
	DeltaUpdates              *bool             `yaml:"delta_updates,omitempty" description:"Appends the changes of each transaction instead of merging them. The connector only supports delta updates, so it rejects false. Defaults to true."`
	AdditionalTableProperties map[string]string `yaml:"additional_table_properties,omitempty" description:"Iceberg table properties set when the table is created (e.g., 'write.parquet.compression-codec' = 'zstd'). They can't be changed afterwards."`
}

// Binding maps a source collection to its Iceberg table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is an Iceberg delta updates materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
