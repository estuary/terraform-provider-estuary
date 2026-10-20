package postgres

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// SSLMode overrides the connection SSL behavior through the PostgreSQL 'sslmode' parameter.
type SSLMode string

// Image is the PostgreSQL connector image, also materializing into Amazon Aurora and RDS for PostgreSQL.
const Image string = "ghcr.io/estuary/materialize-postgres:v5"

// ConnectorName ends the catalog name of every task of the connector, as the dashboard names them.
const ConnectorName string = "materialize-postgres"

const (
	SSLModeDisable    SSLMode = "disable"
	SSLModeAllow      SSLMode = "allow"
	SSLModePrefer     SSLMode = "prefer"
	SSLModeRequire    SSLMode = "require"
	SSLModeVerifyCA   SSLMode = "verify-ca"
	SSLModeVerifyFull SSLMode = "verify-full"
)

// SSLModes lists every supported SSL mode, useful for schema validators.
var SSLModes = []SSLMode{
	SSLModeDisable,
	SSLModeAllow,
	SSLModePrefer,
	SSLModeRequire,
	SSLModeVerifyCA,
	SSLModeVerifyFull,
}

// Advanced carries the advanced endpoint options, including the SSL mode and feature flags.
type Advanced struct {
	SSLMode        SSLMode `yaml:"sslmode,omitempty"`
	NoFlowDocument bool    `yaml:"no_flow_document,omitempty"`
	FeatureFlags   string  `yaml:"feature_flags,omitempty"`
}

// Config carries the ghcr.io/estuary/materialize-postgres connector configurations, including
// the database connection, its authentication and the default schema. Password is the
// deprecated form of Credentials.
type Config struct {
	Address       string                         `yaml:"address"`
	User          string                         `yaml:"user"`
	Password      string                         `yaml:"password,omitempty"`
	Database      string                         `yaml:"database,omitempty"`
	Schema        string                         `yaml:"schema,omitempty"`
	HardDelete    bool                           `yaml:"hardDelete,omitempty"`
	Credentials   *auth.DatabaseCredentials      `yaml:"credentials,omitempty"`
	DBTJobTrigger *materialization.DBTJobTrigger `yaml:"dbt_job_trigger,omitempty"`
	Advanced      *Advanced                      `yaml:"advanced,omitempty"`
	NetworkTunnel *endpoint.NetworkTunnel        `yaml:"networkTunnel,omitempty"`
}

// Resource identifies the target PostgreSQL table, Schema overriding the endpoint schema.
type Resource struct {
	Table                    string `yaml:"table"`
	Schema                   string `yaml:"schema,omitempty"`
	AdditionalTableCreateSQL string `yaml:"additional_table_create_sql,omitempty"`
	DeltaUpdates             bool   `yaml:"delta_updates,omitempty"`
}

// Binding maps a source collection to the PostgreSQL table it materializes into.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full PostgreSQL materialization entry, keyed by its catalog name in a catalog spec.
type Spec = materialization.Spec[EndpointSpec, Resource]
