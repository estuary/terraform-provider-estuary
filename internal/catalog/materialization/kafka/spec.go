package kafka

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/kafka"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// MessageFormat is how materialized messages are encoded.
type MessageFormat string

// Compression is the algorithm materialized messages are compressed with.
type Compression string

// Image is the Kafka materialization connector image.
const Image string = "ghcr.io/estuary/materialize-kafka:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "materialize-kafka"

// AuthTypes lists every supported auth type, useful for schema validators.
var AuthTypes = []kafka.AuthType{
	kafka.AuthTypeUserPassword,
}

const (
	MessageFormatAvro MessageFormat = "Avro"
	MessageFormatJSON MessageFormat = "JSON"
)

// MessageFormats lists every supported message format, useful for schema validators.
var MessageFormats = []MessageFormat{
	MessageFormatAvro,
	MessageFormatJSON,
}

const (
	CompressionNone   Compression = "none"
	CompressionGzip   Compression = "gzip"
	CompressionLZ4    Compression = "lz4"
	CompressionSnappy Compression = "snappy"
	CompressionZstd   Compression = "zstd"
)

// Compressions lists every supported compression, useful for schema validators.
var Compressions = []Compression{
	CompressionNone,
	CompressionGzip,
	CompressionLZ4,
	CompressionSnappy,
	CompressionZstd,
}

// Credentials is the cluster authentication, whose only AuthType is kafka.AuthTypeUserPassword.
type Credentials struct {
	AuthType   kafka.AuthType `yaml:"auth_type" enum:"user_password" description:"Authentication method, which must be 'user_password' (SASL user and password), the only one the connector has."`
	kafka.SASL `yaml:",inline"`
}

// SchemaRegistry is the schema registry Avro schemas are registered in.
type SchemaRegistry struct {
	Endpoint string `yaml:"endpoint" credentials:"forbidden" description:"URL of the schema registry API (e.g., 'https://psrc-12345.us-east-2.aws.confluent.cloud'). The plan rejects credentials in it, such as user:password@ or a password, token, API key or signature option, as state and logs would hold them in plain text: set them in username and password. Credentials found in the live value show as ***."`
	Username string `yaml:"username" description:"User of the schema registry, the key of a Confluent Cloud schema registry API key."`
	Password string `yaml:"password" description:"Password of the schema registry user, the secret of a Confluent Cloud schema registry API key. Write-only."`
}

// Config is the materialize-kafka connector configuration. AvroLogicalTypes is a pointer because the connector's
// default is true, so an explicit false must be published.
type Config struct {
	kafka.Connection       `yaml:",inline"`
	Credentials            Credentials     `yaml:"credentials" description:"How the connector authenticates with the cluster."`
	MessageFormat          MessageFormat   `yaml:"message_format" enum:"Avro,JSON" description:"Format of the materialized messages: 'Avro', which requires schema_registry, or 'JSON', which uses no schema registry."`
	SchemaRegistry         *SchemaRegistry `yaml:"schema_registry,omitempty" description:"Schema registry the Avro schemas of the messages are registered in, required with 'Avro'."`
	Compression            Compression     `yaml:"compression,omitempty" enum:"none,gzip,lz4,snappy,zstd" description:"Compression of the messages: 'none', 'gzip', 'lz4' (default), 'snappy' or 'zstd'. Not every broker supports every algorithm."`
	AvroLogicalTypes       *bool           `yaml:"avro_logical_types,omitempty" description:"Registers Avro schemas with logical types, so date-time fields such as flow_published_at are timestamps instead of strings. Defaults to true. Changing it on an existing materialization changes the type of those fields in its topics, which can break their consumers."`
	TopicPartitions        int             `yaml:"topic_partitions,omitempty" description:"Number of partitions of the topics the connector creates. Defaults to 6."`
	TopicReplicationFactor int             `yaml:"topic_replication_factor,omitempty" description:"Replication factor of the topics the connector creates. Defaults to 3."`
}

// Resource identifies the target Kafka topic.
type Resource struct {
	Topic string `yaml:"topic" x-collection-name:"true" description:"Kafka topic to materialize into, which the connector creates when it doesn't exist."`
}

// Binding maps a source collection to its Kafka topic.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a Kafka materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
