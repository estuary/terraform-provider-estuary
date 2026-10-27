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
	ImageAmazonAurora string = "ghcr.io/estuary/source-amazon-aurora-postgres:v3"
	ImageAmazonRDS    string = "ghcr.io/estuary/source-amazon-rds-postgres:v3"
)

// Images lists every PostgreSQL capture image, useful for schema validators.
var Images = []string{
	ImageAmazonAurora,
	ImageAmazonRDS,
}

// Connector names end the catalog name of the tasks of their image.
const (
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
	IncludeSchemas            []string `yaml:"include_schemas,omitempty"`
	ExcludeSchemas            []string `yaml:"exclude_schemas,omitempty"`
	TablePatterns             []string `yaml:"table_patterns,omitempty"`
	DiscoverUnpublishedTables bool     `yaml:"discover_unpublished_tables,omitempty"`
}

// Advanced carries the advanced endpoint options, including replication, backfill and discovery settings.
type Advanced struct {
	PublicationName           string           `yaml:"publicationName,omitempty"`
	SlotName                  string           `yaml:"slotName,omitempty"`
	WatermarksTable           string           `yaml:"watermarksTable,omitempty"`
	SkipBackfills             string           `yaml:"skip_backfills,omitempty"`
	BackfillChunkSize         int              `yaml:"backfill_chunk_size,omitempty"`
	SSLMode                   sslmode.Postgres `yaml:"sslmode,omitempty" enum:"disable,allow,prefer,require,verify-ca,verify-full"`
	DiscoverSchemas           []string         `yaml:"discover_schemas,omitempty"`
	MinimumBackfillXID        string           `yaml:"min_backfill_xid,omitempty"`
	ReadOnlyCapture           bool             `yaml:"read_only_capture,omitempty"`
	CaptureAsPartitions       bool             `yaml:"capture_as_partitions,omitempty"`
	DiscoverUnpublishedTables bool             `yaml:"discover_unpublished_tables,omitempty"`
	DiscoverOnlyPublished     bool             `yaml:"discover_only_published,omitempty"`
	SourceTag                 string           `yaml:"source_tag,omitempty"`
	RediscoveryInterval       string           `yaml:"rediscovery_interval,omitempty"`
	FeatureFlags              string           `yaml:"feature_flags,omitempty"`
	StatementTimeout          StatementTimeout `yaml:"statement_timeout,omitempty" enum:",0,30s,1m,2m,5m,30m"`
}

// Config is the source-postgres configuration shared by every image in Images. Password is the deprecated form of
// Credentials.
type Config struct {
	Address          string                    `yaml:"address"`
	User             string                    `yaml:"user"`
	Password         string                    `yaml:"password,omitempty"`
	Database         string                    `yaml:"database"`
	HistoryMode      bool                      `yaml:"historyMode"`
	Credentials      *auth.DatabaseCredentials `yaml:"credentials,omitempty"`
	DiscoveryFilters *DiscoveryFilters         `yaml:"discoveryFilters,omitempty"`
	Advanced         *Advanced                 `yaml:"advanced,omitempty"`
	NetworkTunnel    *endpoint.NetworkTunnel   `yaml:"networkTunnel,omitempty"`
}

// Resource identifies the captured table, shared by the SQL captures.
type Resource = sqlcapture.Resource

// Binding maps a captured PostgreSQL resource to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full PostgreSQL capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
