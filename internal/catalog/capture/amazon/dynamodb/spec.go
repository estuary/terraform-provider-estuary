package dynamodb

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// Image is the DynamoDB connector image.
const Image string = "ghcr.io/estuary/source-dynamodb:v1"

// ConnectorName ends the catalog name of the connector tasks.
const ConnectorName string = "source-dynamodb"

// Advanced carries the advanced endpoint options, including the backfill scan and stream polling settings.
type Advanced struct {
	BackfillSegments int    `yaml:"backfillSegments,omitempty"`
	ScanLimit        int    `yaml:"scanLimit,omitempty"`
	IdleBackoffMax   string `yaml:"idleBackoffMax,omitempty"`
	Endpoint         string `yaml:"endpoint,omitempty"`
}

// Config is the source-dynamodb configuration: region and AWS authentication.
type Config struct {
	Region      string              `yaml:"region"`
	Credentials auth.AWSCredentials `yaml:"credentials"`
	Advanced    *Advanced           `yaml:"advanced,omitempty"`
}

// Resource identifies the captured DynamoDB table.
type Resource struct {
	Table         string `yaml:"table"`
	RCUAllocation int    `yaml:"rcuAllocation,omitempty"`
}

// ResourcePath returns the table, satisfying capture.ResourcePather.
func (resource Resource) ResourcePath() []string {
	return []string{resource.Table}
}

// Binding maps a captured DynamoDB resource to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full DynamoDB capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
