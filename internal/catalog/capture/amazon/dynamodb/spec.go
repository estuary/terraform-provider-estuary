package dynamodb

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// Image is the DynamoDB connector image.
const Image string = "ghcr.io/estuary/source-dynamodb:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "source-dynamodb"

// Advanced carries the advanced endpoint options, including the backfill scan and stream polling settings.
type Advanced struct {
	BackfillSegments int    `yaml:"backfillSegments,omitempty" description:"Number of segments the backfill scan of each table is split into. Defaults to 100. Has no effect once the backfill of a table has started."`
	ScanLimit        int    `yaml:"scanLimit,omitempty" description:"Maximum number of items each backfill scan request evaluates. By default, it's unlimited, except for tables whose read capacity, from rcu_allocation or else provisioned, is 100 or less, which use 100."`
	IdleBackoffMax   string `yaml:"idleBackoffMax,omitempty" description:"Maximum wait between polls of an open stream shard whose reads keep returning no records, as a Go duration between '1s' and '10m'. Defaults to '1s'. Higher values reduce billed DynamoDB Streams requests on idle shards, but delay the capture of new records by up to this wait."`
	Endpoint         string `yaml:"endpoint,omitempty" description:"AWS endpoint URI to connect to, for both DynamoDB and DynamoDB Streams. Use it to capture from a DynamoDB-compatible API that AWS doesn't provide."`
}

// Config is the source-dynamodb configuration: region and AWS authentication.
type Config struct {
	Region      string              `yaml:"region" description:"AWS region of the DynamoDB tables (e.g., 'us-east-1')."`
	Credentials auth.AWSCredentials `yaml:"credentials" description:"AWS authentication of the capture."`
	Advanced    *Advanced           `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
}

// Resource identifies the captured DynamoDB table.
type Resource struct {
	Table         string `yaml:"table" description:"Name of the captured table."`
	RCUAllocation int    `yaml:"rcuAllocation,omitempty" description:"Read capacity units the backfill of the table tries to consume. Defaults to the provisioned read capacity of the table, and is unlimited for tables without one, such as on-demand tables."`
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
