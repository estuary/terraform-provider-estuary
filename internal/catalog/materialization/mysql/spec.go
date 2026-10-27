package mysql

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/sslmode"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// Image is the MySQL connector image, also used for Amazon Aurora and RDS for MySQL.
const Image string = "ghcr.io/estuary/materialize-mysql:v2"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "materialize-mysql"

// Advanced holds advanced endpoint options; SSLServerCA is required by verify_ca and verify_identity.
type Advanced struct {
	SSLMode        sslmode.MySQL `yaml:"sslmode,omitempty" enum:"disabled,preferred,required,verify_ca,verify_identity"`
	SSLServerCA    string        `yaml:"ssl_server_ca,omitempty" secret:"true"`
	SSLClientCert  string        `yaml:"ssl_client_cert,omitempty" secret:"true"`
	SSLClientKey   string        `yaml:"ssl_client_key,omitempty"`
	NoFlowDocument bool          `yaml:"no_flow_document,omitempty"`
	FeatureFlags   string        `yaml:"feature_flags,omitempty"`
}

// Config is the materialize-mysql connector configuration.
type Config struct {
	Address       string                         `yaml:"address"`
	User          string                         `yaml:"user"`
	Password      string                         `yaml:"password"`
	Database      string                         `yaml:"database"`
	Timezone      string                         `yaml:"timezone,omitempty"`
	HardDelete    bool                           `yaml:"hardDelete,omitempty"`
	DBTJobTrigger *materialization.DBTJobTrigger `yaml:"dbt_job_trigger,omitempty"`
	Advanced      *Advanced                      `yaml:"advanced,omitempty"`
	NetworkTunnel *endpoint.NetworkTunnel        `yaml:"networkTunnel,omitempty"`
}

// Resource identifies the target MySQL table.
type Resource struct {
	Table                    string `yaml:"table"`
	AdditionalTableCreateSQL string `yaml:"additional_table_create_sql,omitempty"`
	DeltaUpdates             bool   `yaml:"delta_updates,omitempty"`
}

// Binding maps a source collection to its MySQL table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a MySQL materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
