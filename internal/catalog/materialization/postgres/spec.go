package postgres

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/sslmode"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// Images of the connector variants, re-tags of one connector named after the dashboard tiles.
const (
	ImagePostgres     string = "ghcr.io/estuary/materialize-postgres:v5"
	ImageAmazonAurora string = "ghcr.io/estuary/materialize-amazon-aurora-postgres:v5"
	ImageAmazonRDS    string = "ghcr.io/estuary/materialize-amazon-rds-postgres:v5"
)

// Images lists every PostgreSQL materialization image, useful for schema validators.
var Images = []string{
	ImagePostgres,
	ImageAmazonAurora,
	ImageAmazonRDS,
}

// Connector names are the catalog name suffixes of the tasks of each image, as the dashboard names them.
const (
	ConnectorNamePostgres     string = "materialize-postgres"
	ConnectorNameAmazonAurora string = "materialize-amazon-aurora-postgres"
	ConnectorNameAmazonRDS    string = "materialize-amazon-rds-postgres"
)

// Advanced holds advanced endpoint options, such as the SSL mode.
type Advanced struct {
	SSLMode        sslmode.Postgres `yaml:"sslmode,omitempty" enum:"disable,allow,prefer,require,verify-ca,verify-full" description:"Overrides the PostgreSQL sslmode connection parameter: 'disable', 'allow', 'prefer', 'require', 'verify-ca' or 'verify-full'."`
	NoFlowDocument bool             `yaml:"no_flow_document,omitempty" description:"Stops requiring the flow_document field in bindings with standard updates. Defaults to false."`
	FeatureFlags   string           `yaml:"feature_flags,omitempty" description:"Connector settings for edge cases and experimental features, see https://docs.estuary.dev/guides/advanced-usage/feature-flags/. Intended for Estuary internal use: change them only as directed by Estuary support."`
}

// Config is the materialize-postgres configuration shared by every image in Images; Password is the deprecated form of
// Credentials.
type Config struct {
	Address       string                         `yaml:"address" description:"Host or host:port of the database. The port defaults to 5432."`
	User          string                         `yaml:"user" description:"Database user to connect as."`
	Password      string                         `yaml:"password,omitempty" description:"Deprecated form of credentials with auth_type 'UserPassword', still accepted for existing materializations. Write-only."`
	Database      string                         `yaml:"database,omitempty" description:"Logical database to materialize to."`
	Schema        string                         `yaml:"schema,omitempty" description:"Schema of the materialized tables and of the connector's metadata tables, unless a binding sets its own. Defaults to 'public'."`
	HardDelete    bool                           `yaml:"hardDelete,omitempty" description:"Deletes from the table the documents deleted in the source. Otherwise, the _meta/op column marks deleted rows (soft delete). Defaults to false."`
	Credentials   *auth.DatabaseCredentials      `yaml:"credentials,omitempty" description:"How the user authenticates. Set it, or the deprecated password, for the materialization to connect."`
	DBTJobTrigger *materialization.DBTJobTrigger `yaml:"dbt_job_trigger,omitempty" description:"Triggers a dbt Cloud job when new data is materialized."`
	Advanced      *Advanced                      `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
	NetworkTunnel *endpoint.NetworkTunnel        `yaml:"networkTunnel,omitempty" description:"Connects through an SSH server acting as a bastion host."`
}

// Resource identifies the target PostgreSQL table; Schema overrides the endpoint schema.
type Resource struct {
	Table                    string `yaml:"table" description:"Table to materialize into. The connector creates it, unless it created it before."`
	Schema                   string `yaml:"schema,omitempty" description:"Schema of the table, overriding config.schema."`
	AdditionalTableCreateSQL string `yaml:"additional_table_create_sql,omitempty" description:"SQL statements run in the same transaction that creates the table, such as a CREATE INDEX. They only run when the table is created, so an existing table needs a backfill of the binding. Don't re-create the table in them."`
	DeltaUpdates             bool   `yaml:"delta_updates,omitempty" description:"Appends the changes of each transaction instead of merging them into one row per key. Defaults to false."`
}

// Binding maps a source collection to its PostgreSQL table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a PostgreSQL materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
