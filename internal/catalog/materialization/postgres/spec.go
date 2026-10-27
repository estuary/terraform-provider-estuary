package postgres

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/sslmode"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// Image is the PostgreSQL connector image, also used for Amazon Aurora and RDS for PostgreSQL.
const Image string = "ghcr.io/estuary/materialize-postgres:v5"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "materialize-postgres"

// Advanced holds advanced endpoint options, such as the SSL mode.
type Advanced struct {
	SSLMode        sslmode.Postgres `yaml:"sslmode,omitempty" enum:"disable,allow,prefer,require,verify-ca,verify-full"`
	NoFlowDocument bool             `yaml:"no_flow_document,omitempty"`
	FeatureFlags   string           `yaml:"feature_flags,omitempty"`
}

// Config is the materialize-postgres connector configuration; Password is the deprecated form of Credentials.
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

// Resource identifies the target PostgreSQL table; Schema overrides the endpoint schema.
type Resource struct {
	Table                    string `yaml:"table"`
	Schema                   string `yaml:"schema,omitempty"`
	AdditionalTableCreateSQL string `yaml:"additional_table_create_sql,omitempty"`
	DeltaUpdates             bool   `yaml:"delta_updates,omitempty"`
}

// Binding maps a source collection to its PostgreSQL table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a PostgreSQL materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
