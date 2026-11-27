package redshiftbatch

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/sqlbatch"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/sslmode"
)

// Image is the Redshift batch connector image.
const Image string = "ghcr.io/estuary/source-redshift-batch:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "source-redshift-batch"

// Advanced holds the advanced endpoint options: the ones every batch SQL capture has, and Redshift's.
type Advanced struct {
	sqlbatch.Advanced `yaml:",inline"`
	SSLMode           sslmode.Postgres `yaml:"sslmode,omitempty" enum:"disable,allow,prefer,require,verify-ca,verify-full" description:"Overrides the sslmode connection parameter: 'disable', 'allow', 'prefer', 'require', 'verify-ca' or 'verify-full'."`
}

// Config is the source-redshift-batch connector configuration.
type Config struct {
	sqlbatch.Connection `yaml:",inline"`
	Database            string                  `yaml:"database" description:"Database to capture from. The connector's own default is 'dev'."`
	NetworkTunnel       *sqlbatch.NetworkTunnel `yaml:"networkTunnel,omitempty" description:"Connects through an SSH server acting as a bastion host."`
	Advanced            *Advanced               `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
}

// Resource is the query a binding polls.
type Resource = sqlbatch.Resource

// Binding maps a polled query to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Redshift batch capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
