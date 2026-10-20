package dynamodb

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// Image is the DynamoDB connector image.
const Image string = "ghcr.io/estuary/materialize-dynamodb:v1"

// ConnectorName ends the catalog name of every task of the connector, as the dashboard names them.
const ConnectorName string = "materialize-dynamodb"

// Advanced carries the advanced endpoint options, including a custom AWS endpoint and feature flags.
type Advanced struct {
	Endpoint     string `yaml:"endpoint,omitempty"`
	FeatureFlags string `yaml:"feature_flags,omitempty"`
}

// Config carries the ghcr.io/estuary/materialize-dynamodb connector configurations, including
// the tables region and the AWS authentication.
type Config struct {
	Region      string              `yaml:"region"`
	Credentials auth.AWSCredentials `yaml:"credentials"`
	Advanced    *Advanced           `yaml:"advanced,omitempty"`
}

// Resource identifies the target DynamoDB table, created by the connector when missing.
type Resource struct {
	Table string `yaml:"table"`
}

// Binding maps a source collection to the DynamoDB table it materializes into.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full DynamoDB materialization entry, keyed by its catalog name in a catalog spec.
type Spec = materialization.Spec[EndpointSpec, Resource]
