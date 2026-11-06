package mysql

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/sslmode"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// Images of the connector variants, re-tags of one connector named after the dashboard tiles.
const (
	ImageMySQL        string = "ghcr.io/estuary/materialize-mysql:v2"
	ImageAmazonAurora string = "ghcr.io/estuary/materialize-amazon-aurora-mysql:v2"
	ImageAmazonRDS    string = "ghcr.io/estuary/materialize-amazon-rds-mysql:v2"
)

// Connector names are the catalog name suffixes of the tasks of each image, as the dashboard names them.
const (
	ConnectorNameMySQL        string = "materialize-mysql"
	ConnectorNameAmazonAurora string = "materialize-amazon-aurora-mysql"
	ConnectorNameAmazonRDS    string = "materialize-amazon-rds-mysql"
)

// Advanced holds advanced endpoint options, including the TLS settings.
type Advanced struct {
	sslmode.MySQLTLS `yaml:",inline"`
	NoFlowDocument   bool   `yaml:"no_flow_document,omitempty" description:"Stops requiring the flow_document field in bindings with standard updates. Defaults to false."`
	FeatureFlags     string `yaml:"feature_flags,omitempty" description:"Connector settings for edge cases and experimental features, see https://docs.estuary.dev/guides/advanced-usage/feature-flags/. Intended for Estuary internal use: change them only as directed by Estuary support."`
}

// Config is the materialize-mysql configuration shared by every image variant.
type Config struct {
	Address       string                         `yaml:"address" credentials:"forbidden" description:"Host or host:port of the database. The port defaults to 3306. The plan rejects credentials in it, such as user:password@ or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as ***."`
	User          string                         `yaml:"user" description:"Database user to connect as. It needs permission to create tables."`
	Password      string                         `yaml:"password" description:"Password of the database user. Write-only."`
	Database      string                         `yaml:"database" description:"Logical database to materialize to."`
	Timezone      string                         `yaml:"timezone,omitempty" description:"Time zone of the materialized datetime columns: an IANA time zone name (e.g., 'America/New_York') or a +HH:MM offset. Leave it unset to use the database time_zone system variable, which it overrides when both are set. Required when time_zone is SYSTEM, as the connector can't read the time zone then."`
	HardDelete    bool                           `yaml:"hardDelete,omitempty" description:"Deletes from the table the documents deleted in the source. Otherwise, the _meta/op column marks deleted rows (soft delete). Defaults to false."`
	DBTJobTrigger *materialization.DBTJobTrigger `yaml:"dbt_job_trigger,omitempty" description:"Triggers a dbt Cloud job when new data is materialized."`
	Advanced      *Advanced                      `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
	NetworkTunnel *endpoint.NetworkTunnel        `yaml:"networkTunnel,omitempty" description:"Connects through an SSH server acting as a bastion host."`
}

// Resource identifies the target MySQL table.
type Resource struct {
	Table                    string `yaml:"table" x-collection-name:"true" description:"Table to materialize into. The connector creates it, unless it created it before."`
	AdditionalTableCreateSQL string `yaml:"additional_table_create_sql,omitempty" description:"SQL statements run after the connector creates the table, such as a CREATE INDEX. They only run when the connector creates the table: a backfill truncates an existing table without running them, so run them yourself on it. Don't re-create the table in them."`
	DeltaUpdates             bool   `yaml:"delta_updates,omitempty" x-delta-updates:"true" description:"Appends the changes of each transaction instead of merging them into one row per key. Defaults to false. Switching it from true to false requires a backfill: in the same change, set or raise backfill.trigger, or set or raise the materialization's, which backfills every enabled binding, or the plan fails."`
}

// Binding maps a source collection to its MySQL table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a MySQL materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
