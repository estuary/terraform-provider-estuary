package kafka

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/kafka"
)

// SchemaRegistryType is whether the capture decodes messages through a schema registry.
type SchemaRegistryType string

// Image is the Kafka capture connector image.
const Image string = "ghcr.io/estuary/source-kafka:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "source-kafka"

// AuthTypeAWS authenticates with AWS MSK through IAM, which only the capture connector has.
const AuthTypeAWS kafka.AuthType = "AWS"

// AuthTypes lists every supported auth type, useful for schema validators.
var AuthTypes = []kafka.AuthType{
	kafka.AuthTypeUserPassword,
	AuthTypeAWS,
}

const (
	SchemaRegistryTypeConfluent SchemaRegistryType = "confluent_schema_registry"
	SchemaRegistryTypeNone      SchemaRegistryType = "no_schema_registry"
)

// SchemaRegistryTypes lists every supported schema registry type, useful for schema validators.
var SchemaRegistryTypes = []SchemaRegistryType{
	SchemaRegistryTypeConfluent,
	SchemaRegistryTypeNone,
}

// Credentials is the cluster authentication, with the fields of its AuthType.
type Credentials struct {
	AuthType           kafka.AuthType `yaml:"auth_type" enum:"user_password,AWS" description:"Authentication method: 'user_password' (SASL user and password) or 'AWS' (IAM access key of an Amazon MSK cluster). Only the attributes of the selected method are used."`
	kafka.SASL         `yaml:",inline"`
	AWSAccessKeyID     string `yaml:"aws_access_key_id,omitempty" description:"AWS access key ID of the IAM user the connector authenticates as, with 'AWS'."`
	AWSSecretAccessKey string `yaml:"aws_secret_access_key,omitempty" description:"AWS secret access key of the IAM user, with 'AWS'. Write-only."`
	Region             string `yaml:"region,omitempty" description:"AWS region of the MSK cluster (e.g., 'us-east-1'), with 'AWS'."`
}

// SchemaRegistry is the schema registry the capture decodes Avro and Protobuf messages with, or its absence, with the
// fields of its Type. EnableJSONOnly is a pointer so that no_schema_registry can publish an explicit false, which the
// connector requires the key for.
type SchemaRegistry struct {
	Type           SchemaRegistryType `yaml:"schema_registry_type" enum:"confluent_schema_registry,no_schema_registry" description:"Whether messages are decoded through a schema registry: 'confluent_schema_registry' uses the registry at endpoint, 'no_schema_registry' uses none. Only the attributes of the selected type are used."`
	Endpoint       string             `yaml:"endpoint,omitempty" credentials:"forbidden" description:"URL of the schema registry API (e.g., 'https://psrc-12345.us-east-2.aws.confluent.cloud'), with 'confluent_schema_registry'. The plan rejects credentials in it, such as user:password@ or a password, token, API key or signature option, as state and logs would hold them in plain text: set them in username and password. Credentials found in the live value show as ***."`
	Username       string             `yaml:"username,omitempty" description:"User of the schema registry, the key of a Confluent Cloud schema registry API key, with 'confluent_schema_registry'."`
	Password       string             `yaml:"password,omitempty" description:"Password of the schema registry user, the secret of a Confluent Cloud schema registry API key, with 'confluent_schema_registry'. Write-only."`
	EnableJSONOnly *bool              `yaml:"enable_json_only,omitempty" description:"Confirms that every message is captured as JSON, with 'no_schema_registry', which requires it to be true. Every topic is then discovered, with collections keyed by message partition and offset."`
}

// Config is the source-kafka connector configuration.
type Config struct {
	kafka.Connection `yaml:",inline"`
	Credentials      Credentials    `yaml:"credentials" description:"How the connector authenticates with the cluster."`
	SchemaRegistry   SchemaRegistry `yaml:"schema_registry" description:"Schema registry the messages are decoded with, or none."`
}

// Resource identifies the captured Kafka topic.
type Resource struct {
	Topic string `yaml:"topic" description:"Kafka topic the binding captures messages from."`
}

// ResourcePath returns the topic, satisfying capture.ResourcePather.
func (resource Resource) ResourcePath() []string {
	return []string{resource.Topic}
}

// Binding maps a captured Kafka topic to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Kafka capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
