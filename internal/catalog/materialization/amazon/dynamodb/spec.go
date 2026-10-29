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
	Endpoint     string `yaml:"endpoint,omitempty" description:"AWS endpoint URI to connect to. Use it to materialize to a DynamoDB-compatible API that AWS doesn't provide."`
	FeatureFlags string `yaml:"feature_flags,omitempty" description:"Connector settings for edge cases and experimental features, see https://docs.estuary.dev/guides/advanced-usage/feature-flags/. Intended for Estuary internal use: change them only as directed by Estuary support."`
}

// Config is the materialize-dynamodb connector configuration.
type Config struct {
	Region      string              `yaml:"region" description:"AWS region of the DynamoDB tables (e.g., 'us-east-1')."`
	Credentials auth.AWSCredentials `yaml:"credentials" description:"AWS authentication of the materialization."`
	Advanced    *Advanced           `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
}

// Resource identifies the target DynamoDB table, which the connector creates when missing.
type Resource struct {
	Table string `yaml:"table" description:"DynamoDB table to materialize into. The connector creates it when missing."`
}

// Binding maps a source collection to its DynamoDB table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a DynamoDB materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
