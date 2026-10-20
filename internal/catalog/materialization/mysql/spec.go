package mysql

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// SSLMode controls whether connections use TLS and how strictly the server certificate is verified.
type SSLMode string

// Image is the MySQL connector image, also materializing into Amazon Aurora and RDS for MySQL.
const Image string = "ghcr.io/estuary/materialize-mysql:v2"

// ConnectorName ends the catalog name of every task of the connector, as the dashboard names them.
const ConnectorName string = "materialize-mysql"

const (
	SSLModeDisabled       SSLMode = "disabled"
	SSLModePreferred      SSLMode = "preferred"
	SSLModeRequired       SSLMode = "required"
	SSLModeVerifyCA       SSLMode = "verify_ca"
	SSLModeVerifyIdentity SSLMode = "verify_identity"
)

// SSLModes lists every supported SSL mode, useful for schema validators.
var SSLModes = []SSLMode{
	SSLModeDisabled,
	SSLModePreferred,
	SSLModeRequired,
	SSLModeVerifyCA,
	SSLModeVerifyIdentity,
}

// Advanced carries the advanced endpoint options, including the TLS settings and feature flags.
// SSLServerCA is required by the verify_ca and verify_identity SSL modes.
type Advanced struct {
	SSLMode        SSLMode `yaml:"sslmode,omitempty"`
	SSLServerCA    string  `yaml:"ssl_server_ca,omitempty"`
	SSLClientCert  string  `yaml:"ssl_client_cert,omitempty"`
	SSLClientKey   string  `yaml:"ssl_client_key,omitempty"`
	NoFlowDocument bool    `yaml:"no_flow_document,omitempty"`
	FeatureFlags   string  `yaml:"feature_flags,omitempty"`
}

// Config carries the ghcr.io/estuary/materialize-mysql connector configurations, including
// the database connection, its password and the timezone of materialized datetime columns.
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

// Binding maps a source collection to the MySQL table it materializes into.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full MySQL materialization entry, keyed by its catalog name in a catalog spec.
type Spec = materialization.Spec[EndpointSpec, Resource]
