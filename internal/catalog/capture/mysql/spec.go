package mysql

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/sqlcapture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/sslmode"
)

// StatementTimeout overrides the connector statement timeout, blank disables it.
type StatementTimeout string

// Images of the connector variants, re-tags of one connector named after the dashboard tiles.
const (
	ImageAmazonAurora string = "ghcr.io/estuary/source-amazon-aurora-mysql:v3"
	ImageAmazonRDS    string = "ghcr.io/estuary/source-amazon-rds-mysql:v3"
)

// Images lists every MySQL capture image, useful for schema validators.
var Images = []string{
	ImageAmazonAurora,
	ImageAmazonRDS,
}

// Connector names are the catalog name suffixes of the tasks of each image, as the dashboard names them.
const (
	ConnectorNameAmazonAurora string = "source-amazon-aurora-mysql"
	ConnectorNameAmazonRDS    string = "source-amazon-rds-mysql"
)

const (
	StatementTimeoutDisabled StatementTimeout = ""
	StatementTimeout30s      StatementTimeout = "30s"
	StatementTimeout1m       StatementTimeout = "1m"
	StatementTimeout5m       StatementTimeout = "5m"
	StatementTimeout30m      StatementTimeout = "30m"
)

// StatementTimeouts lists every supported statement timeout, useful for schema validators.
var StatementTimeouts = []StatementTimeout{
	StatementTimeoutDisabled,
	StatementTimeout30s,
	StatementTimeout1m,
	StatementTimeout5m,
	StatementTimeout30m,
}

// DiscoveryFilters restricts which tables are visible to discovery.
type DiscoveryFilters struct {
	IncludeSchemas []string `yaml:"include_schemas,omitempty" description:"Only discovers tables in these databases. Combined with advanced.discover_schemas."`
	ExcludeSchemas []string `yaml:"exclude_schemas,omitempty" description:"Never discovers tables in these databases."`
	TablePatterns  []string `yaml:"table_patterns,omitempty" description:"Only discovers tables matching at least one of these glob patterns, using * and ? as wildcards. A pattern with a . matches database.table (e.g., 'shop.orders_*'), a pattern without one matches the table name in any database."`
}

// Advanced carries the advanced endpoint options, including backfill, discovery and TLS settings.
// WatermarksTable and HeartbeatInterval are deprecated and have no effect.
type Advanced struct {
	DBName                   string           `yaml:"dbname,omitempty" description:"Database to connect to, which doesn't limit what is captured. Defaults to 'mysql'."`
	SkipBinlogRetentionCheck bool             `yaml:"skip_binlog_retention_check,omitempty" description:"Skips the startup check for a dangerously short binary log retention. Only set it if you understand the risk of losing changes."`
	NodeID                   uint32           `yaml:"node_id,omitempty" description:"Replication node ID of the capture, an unsigned 32-bit integer unique within the replication cluster. Derived from the capture name when unset."`
	SkipBackfills            string           `yaml:"skip_backfills,omitempty" description:"Comma-separated list of tables not to backfill, each as database.table."`
	BackfillChunkSize        int              `yaml:"backfill_chunk_size,omitempty" description:"Number of rows fetched in a single backfill query. Defaults to 50000."`
	DiscoverSchemas          []string         `yaml:"discover_schemas,omitempty" description:"Only discovers tables in these databases. Prefer discovery_filters.include_schemas."`
	SourceTag                string           `yaml:"source_tag,omitempty" description:"Value added as the tag property of each document's source metadata."`
	RediscoveryInterval      string           `yaml:"rediscovery_interval,omitempty" description:"How often the running capture checks for schema changes and new tables, as a number and one unit ('ms', 's', 'm' or 'h') from '1m' to '8760h', such as '15m' or '1h'. Defaults to '15m'."`
	FeatureFlags             string           `yaml:"feature_flags,omitempty" description:"Connector settings for edge cases and experimental features, see https://docs.estuary.dev/guides/advanced-usage/feature-flags/. Intended for Estuary internal use: change them only as directed by Estuary support."`
	StatementTimeout         StatementTimeout `yaml:"statement_timeout,omitempty" enum:",30s,1m,5m,30m" description:"Statement timeout of the connector's queries: '30s', '1m', '5m' or '30m'. No timeout when unset."`
	sslmode.MySQLTLS         `yaml:",inline"`
	WatermarksTable          string `yaml:"watermarks_table,omitempty" description:"Deprecated. Only excludes the named table from discovery, kept for existing captures. Defaults to 'flow.watermarks'."`
	HeartbeatInterval        string `yaml:"heartbeat_interval,omitempty" description:"Deprecated and has no effect, kept for existing captures. Defaults to '60s'."`
}

// Config is the source-mysql configuration shared by every image in Images. Password is the deprecated form of
// Credentials.
type Config struct {
	Address          string                    `yaml:"address" credentials:"forbidden" description:"Host or host:port of the database endpoint. The port defaults to 3306. The plan rejects credentials in it, such as user:password@ or a password or token option, as state and logs would hold them in plain text. Credentials found in the live value show as ***."`
	User             string                    `yaml:"user" description:"Database user to authenticate as, usually 'flow_capture'."`
	Password         string                    `yaml:"password,omitempty" description:"Deprecated form of credentials with auth_type 'UserPassword', still accepted for existing captures. At most 32 characters. Write-only."`
	Credentials      *auth.DatabaseCredentials `yaml:"credentials,omitempty" description:"How the user authenticates. Set it, or the deprecated password; the connector rejects a configuration with neither."`
	Timezone         string                    `yaml:"timezone,omitempty" description:"Time zone of DATETIME columns, as an IANA name (e.g., 'America/New_York') or a +HH:MM offset. Leave it unset to use the database's time_zone system variable, which it overrides when set."`
	HistoryMode      bool                      `yaml:"historyMode" description:"Captures every change event as its own document instead of reducing them to the latest state of each row."`
	DiscoveryFilters *DiscoveryFilters         `yaml:"discoveryFilters,omitempty" description:"Restricts which tables discovery finds."`
	Advanced         *Advanced                 `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
	NetworkTunnel    *endpoint.NetworkTunnel   `yaml:"networkTunnel,omitempty" description:"Connects through an SSH server acting as a bastion host."`
}

// Resource identifies the captured table, shared by the SQL captures.
type Resource = sqlcapture.Resource

// Binding maps a captured MySQL resource to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full MySQL capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
