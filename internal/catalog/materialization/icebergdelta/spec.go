package icebergdelta

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
	AuthType           auth.AWSAuthType `yaml:"auth_type" enum:"AWSAccessKey,AWSIAM"`
	AWSAccessKeyID     string           `yaml:"awsAccessKeyId,omitempty"`
	AWSSecretAccessKey string           `yaml:"awsSecretAccessKey,omitempty"`
	AWSRoleARN         string           `yaml:"aws_role_arn,omitempty"`
	AWSRegion          string           `yaml:"aws_region,omitempty"`
}

// CatalogConfig holds the Iceberg catalog settings, using the fields of its CatalogType.
type CatalogConfig struct {
	CatalogType CatalogType `yaml:"catalog_type" enum:"AWS Glue,Iceberg REST Server"`
	GlueID      string      `yaml:"glue_id,omitempty"`
	URI         string      `yaml:"uri,omitempty"`
	Warehouse   string      `yaml:"warehouse,omitempty"`
	Credential  string      `yaml:"credential,omitempty"`
	Token       string      `yaml:"token,omitempty"`
	Scope       string      `yaml:"scope,omitempty"`
}

// Advanced holds advanced endpoint options, such as Iceberg format v3 types.
type Advanced struct {
	NanosecondTimestamps bool   `yaml:"nanosecond_timestamps,omitempty"`
	VariantColumns       bool   `yaml:"variant_columns,omitempty"`
	FeatureFlags         string `yaml:"feature_flags,omitempty"`
}

// Config is the materialize-s3-iceberg connector configuration.
// AWSAccessKeyID and AWSSecretAccessKey are the deprecated form of Credentials.
type Config struct {
	Credentials        *Credentials  `yaml:"credentials,omitempty"`
	AWSAccessKeyID     string        `yaml:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string        `yaml:"aws_secret_access_key,omitempty"`
	Bucket             string        `yaml:"bucket"`
	Prefix             string        `yaml:"prefix,omitempty"`
	Region             string        `yaml:"region"`
	Namespace          string        `yaml:"namespace"`
	UploadInterval     string        `yaml:"upload_interval,omitempty"`
	S3Endpoint         string        `yaml:"s3_endpoint,omitempty"`
	Catalog            CatalogConfig `yaml:"catalog"`
	Advanced           *Advanced     `yaml:"advanced,omitempty"`
}

// Resource identifies the target Iceberg table; the connector only supports delta updates.
type Resource struct {
	Table                     string            `yaml:"table"`
	Namespace                 string            `yaml:"namespace,omitempty"`
	DeltaUpdates              bool              `yaml:"delta_updates,omitempty"`
	AdditionalTableProperties map[string]string `yaml:"additional_table_properties,omitempty"`
}

// Binding maps a source collection to its Iceberg table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is an Iceberg delta updates materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
