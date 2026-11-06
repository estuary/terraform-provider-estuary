package postgres

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/sqlcapture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/sslmode"
)

// StatementTimeout overrides the connector statement timeout, blank uses the 2 minutes default.
type StatementTimeout string

// Images of the connector variants, re-tags of one connector named after the dashboard tiles.
const (
	ImagePostgres     string = "ghcr.io/estuary/source-postgres:v3"
	ImageAmazonAurora string = "ghcr.io/estuary/source-amazon-aurora-postgres:v3"
	ImageAmazonRDS    string = "ghcr.io/estuary/source-amazon-rds-postgres:v3"
)

// Connector names are the catalog name suffixes of the tasks of each image, as the dashboard names them.
const (
	ConnectorNamePostgres     string = "source-postgres"
	ConnectorNameAmazonAurora string = "source-amazon-aurora-postgres"
	ConnectorNameAmazonRDS    string = "source-amazon-rds-postgres"
)

const (
	StatementTimeoutDefault  StatementTimeout = ""
	StatementTimeoutDisabled StatementTimeout = "0"
	StatementTimeout30s      StatementTimeout = "30s"
	StatementTimeout1m       StatementTimeout = "1m"
	StatementTimeout2m       StatementTimeout = "2m"
	StatementTimeout5m       StatementTimeout = "5m"
	StatementTimeout30m      StatementTimeout = "30m"
)

// StatementTimeouts lists every supported statement timeout, useful for schema validators.
var StatementTimeouts = []StatementTimeout{
	StatementTimeoutDefault,
	StatementTimeoutDisabled,
	StatementTimeout30s,
	StatementTimeout1m,
	StatementTimeout2m,
	StatementTimeout5m,
	StatementTimeout30m,
}

// DiscoveryFilters restricts which tables are visible to discovery.
type DiscoveryFilters struct {
	IncludeSchemas            []string `yaml:"include_schemas,omitempty" description:"Only discovers tables in these schemas. Combined with advanced.discover_schemas."`
	ExcludeSchemas            []string `yaml:"exclude_schemas,omitempty" description:"Never discovers tables in these schemas."`
	TablePatterns             []string `yaml:"table_patterns,omitempty" description:"Only discovers tables matching at least one of these glob patterns, using * and ? as wildcards. A pattern with a . matches schema.table (e.g., 'public.orders_*'), a pattern without one matches the table name in any schema."`
	DiscoverUnpublishedTables bool     `yaml:"discover_unpublished_tables,omitempty" description:"Also discovers the tables missing from the publication, adding them to it at capture time when the user has the permissions to. Combined with advanced.discover_unpublished_tables."`
}

// Advanced carries the advanced endpoint options, including replication, backfill and discovery settings.
type Advanced struct {
	PublicationName           string           `yaml:"publicationName,omitempty" description:"PostgreSQL publication to replicate from. Defaults to 'flow_publication'."`
	SlotName                  string           `yaml:"slotName,omitempty" description:"PostgreSQL replication slot to replicate from. Defaults to 'flow_slot'. Each capture of the same database needs its own slot."`
	WatermarksTable           string           `yaml:"watermarksTable,omitempty" description:"Table the connector writes watermarks to during backfills, in schema.table form. Defaults to 'public.flow_watermarks'."`
	SkipBackfills             string           `yaml:"skip_backfills,omitempty" description:"Comma-separated list of tables not to backfill, each as schema.table."`
	BackfillChunkSize         int              `yaml:"backfill_chunk_size,omitempty" description:"Number of rows fetched in a single backfill query. Defaults to 50000."`
	SSLMode                   sslmode.Postgres `yaml:"sslmode,omitempty" enum:"disable,allow,prefer,require,verify-ca,verify-full" description:"Overrides the PostgreSQL sslmode connection parameter: 'disable', 'allow', 'prefer', 'require', 'verify-ca' or 'verify-full'."`
	DiscoverSchemas           []string         `yaml:"discover_schemas,omitempty" description:"Only discovers tables in these schemas. Prefer discovery_filters.include_schemas."`
	MinimumBackfillXID        string           `yaml:"min_backfill_xid,omitempty" description:"Only backfills the rows whose xmin transaction ID is greater than this one in a 32-bit modular comparison, as digits only (e.g., '123456'). Reduces the rows a backfill captures again, though the table is still scanned."`
	ReadOnlyCapture           bool             `yaml:"read_only_capture,omitempty" description:"Runs without writing to the database, so no watermarks table is needed. At least one captured table, or a heartbeat table you add to the capture, must then change every few minutes, or WAL retention can grow without bound."`
	CaptureAsPartitions       bool             `yaml:"capture_as_partitions,omitempty" description:"Discovers and captures each partition of a partitioned table separately, instead of the whole table under its root. Requires a publication created without publish_via_partition_root."`
	DiscoverUnpublishedTables bool             `yaml:"discover_unpublished_tables,omitempty" description:"Same as discovery_filters.discover_unpublished_tables."`
	DiscoverOnlyPublished     bool             `yaml:"discover_only_published,omitempty" description:"Deprecated and has no effect, kept for existing captures."`
	SourceTag                 string           `yaml:"source_tag,omitempty" description:"Value added as the tag property of each document's source metadata."`
	RediscoveryInterval       string           `yaml:"rediscovery_interval,omitempty" description:"How often the running capture checks for schema changes and new tables, as a number and one unit ('ms', 's', 'm' or 'h') from '1m' to '8760h', such as '15m' or '1h'. Defaults to '15m'."`
	FeatureFlags              string           `yaml:"feature_flags,omitempty" description:"Connector settings for edge cases and experimental features, see https://docs.estuary.dev/guides/advanced-usage/feature-flags/. Intended for Estuary internal use: change them only as directed by Estuary support."`
	StatementTimeout          StatementTimeout `yaml:"statement_timeout,omitempty" enum:",0,30s,1m,2m,5m,30m" description:"Statement timeout of the connector's queries: '30s', '1m', '2m', '5m', '30m', or '0' to disable it. Defaults to '2m' when unset."`
}

// Config is the source-postgres configuration shared by every image variant. Password is the deprecated form of
// Credentials.
type Config struct {
	Address          string                    `yaml:"address" credentials:"forbidden" description:"Host or host:port of the database. The port defaults to 5432. The plan rejects credentials in it, such as user:password@ or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as ***."`
	User             string                    `yaml:"user" description:"Database user to authenticate as, usually 'flow_capture'."`
	Password         string                    `yaml:"password,omitempty" description:"Deprecated form of credentials with auth_type 'UserPassword', still accepted for existing captures. Write-only."`
	Database         string                    `yaml:"database" description:"Logical database to capture from, usually 'postgres'."`
	HistoryMode      bool                      `yaml:"historyMode" description:"Captures every change event as its own document instead of reducing them to the latest state of each row."`
	Credentials      *auth.DatabaseCredentials `yaml:"credentials,omitempty" description:"How the user authenticates. Set it, or the deprecated password, for the capture to connect."`
	DiscoveryFilters *DiscoveryFilters         `yaml:"discoveryFilters,omitempty" description:"Restricts which tables discovery finds."`
	Advanced         *Advanced                 `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
	NetworkTunnel    *endpoint.NetworkTunnel   `yaml:"networkTunnel,omitempty" description:"Connects through an SSH server acting as a bastion host."`
}

// Resource identifies the captured table, shared by the SQL captures.
type Resource = sqlcapture.Resource

// Binding maps a captured PostgreSQL resource to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full PostgreSQL capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
