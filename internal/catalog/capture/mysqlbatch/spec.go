package mysqlbatch

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/sqlbatch"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/sslmode"
)

// Image is the MySQL batch connector image.
const Image string = "ghcr.io/estuary/source-mysql-batch:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "source-mysql-batch"

// Advanced holds the advanced endpoint options: the ones every batch SQL capture has, and MySQL's.
type Advanced struct {
	sqlbatch.Advanced `yaml:",inline"`
	DBName            string `yaml:"dbname,omitempty" description:"Database to connect to. It rarely matters, as the connector discovers and captures from every database its user can access."`
	sslmode.MySQLTLS  `yaml:",inline"`
}

// Config is the source-mysql-batch connector configuration.
type Config struct {
	sqlbatch.Connection `yaml:",inline"`
	NetworkTunnel       *sqlbatch.NetworkTunnel `yaml:"networkTunnel,omitempty" description:"Connects through an SSH server acting as a bastion host."`
	Advanced            *Advanced               `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
}

// Resource is the query a binding polls.
type Resource = sqlbatch.Resource

// Binding maps a polled query to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full MySQL batch capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
