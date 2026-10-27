package dynamodb

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// Image is the DynamoDB connector image.
const Image string = "ghcr.io/estuary/materialize-dynamodb:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "materialize-dynamodb"

// Advanced holds advanced endpoint options, such as a custom AWS endpoint.
type Advanced struct {
	Endpoint     string `yaml:"endpoint,omitempty"`
	FeatureFlags string `yaml:"feature_flags,omitempty"`
}

// Config is the materialize-dynamodb connector configuration.
type Config struct {
	Region      string              `yaml:"region"`
	Credentials auth.AWSCredentials `yaml:"credentials"`
	Advanced    *Advanced           `yaml:"advanced,omitempty"`
}

// Resource identifies the target DynamoDB table, which the connector creates when missing.
type Resource struct {
	Table string `yaml:"table"`
}

// Binding maps a source collection to its DynamoDB table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a DynamoDB materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
