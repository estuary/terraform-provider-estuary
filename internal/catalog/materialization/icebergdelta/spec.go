package icebergdelta

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// AuthType identifies how the connector authenticates against AWS.
type AuthType string

// CatalogType identifies the Iceberg catalog implementation.
type CatalogType string

// Image is the Iceberg delta updates connector image.
const Image string = "ghcr.io/estuary/materialize-s3-iceberg:v2"

// ConnectorName ends the catalog name of every task of the connector, as the dashboard names them.
const ConnectorName string = "materialize-s3-iceberg"

const (
	AuthTypeAWSAccessKey AuthType = "AWSAccessKey"
	AuthTypeAWSIAM       AuthType = "AWSIAM"
)

// AuthTypes lists every supported auth type, useful for schema validators.
var AuthTypes = []AuthType{
	AuthTypeAWSAccessKey,
	AuthTypeAWSIAM,
}

const (
	CatalogTypeGlue CatalogType = "AWS Glue"
	CatalogTypeREST CatalogType = "Iceberg REST Server"
)

// CatalogTypes lists every supported catalog type, useful for schema validators.
var CatalogTypes = []CatalogType{
	CatalogTypeGlue,
	CatalogTypeREST,
}

// Credentials carries the AWS authentication, with the fields relevant to its AuthType.
type Credentials struct {
	AuthType           AuthType `yaml:"auth_type"`
	AWSAccessKeyID     string   `yaml:"awsAccessKeyId,omitempty"`
	AWSSecretAccessKey string   `yaml:"awsSecretAccessKey,omitempty"`
	AWSRoleARN         string   `yaml:"aws_role_arn,omitempty"`
	AWSRegion          string   `yaml:"aws_region,omitempty"`
}

// CatalogConfig carries the Iceberg catalog settings, with the fields relevant to its CatalogType.
type CatalogConfig struct {
	CatalogType CatalogType `yaml:"catalog_type"`
	GlueID      string      `yaml:"glue_id,omitempty"`
	URI         string      `yaml:"uri,omitempty"`
	Warehouse   string      `yaml:"warehouse,omitempty"`
	Credential  string      `yaml:"credential,omitempty"`
	Token       string      `yaml:"token,omitempty"`
	Scope       string      `yaml:"scope,omitempty"`
}

// Advanced carries the advanced endpoint options, including the Iceberg format v3 types and feature flags.
type Advanced struct {
	NanosecondTimestamps bool   `yaml:"nanosecond_timestamps,omitempty"`
	VariantColumns       bool   `yaml:"variant_columns,omitempty"`
	FeatureFlags         string `yaml:"feature_flags,omitempty"`
}

// Config carries the ghcr.io/estuary/materialize-s3-iceberg connector configurations, including
// the AWS authentication, the S3 storage and the Iceberg catalog. AWSAccessKeyID and
// AWSSecretAccessKey are the deprecated form of Credentials.
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

// Resource identifies the target Iceberg table, the connector only supports delta updates.
type Resource struct {
	Table                     string            `yaml:"table"`
	Namespace                 string            `yaml:"namespace,omitempty"`
	DeltaUpdates              bool              `yaml:"delta_updates,omitempty"`
	AdditionalTableProperties map[string]string `yaml:"additional_table_properties,omitempty"`
}

// Binding maps a source collection to the Iceberg table it materializes into.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Iceberg delta updates materialization entry, keyed by its catalog name in a catalog spec.
type Spec = materialization.Spec[EndpointSpec, Resource]
