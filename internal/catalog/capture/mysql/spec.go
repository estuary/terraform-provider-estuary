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

// Connector names end the catalog name of the tasks of their image.
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
	IncludeSchemas []string `yaml:"include_schemas,omitempty"`
	ExcludeSchemas []string `yaml:"exclude_schemas,omitempty"`
	TablePatterns  []string `yaml:"table_patterns,omitempty"`
}

// Advanced carries the advanced endpoint options, including backfill, discovery and TLS settings.
// WatermarksTable and HeartbeatInterval are deprecated and have no effect.
type Advanced struct {
	DBName                   string           `yaml:"dbname,omitempty"`
	SkipBinlogRetentionCheck bool             `yaml:"skip_binlog_retention_check,omitempty"`
	NodeID                   uint32           `yaml:"node_id,omitempty"`
	SkipBackfills            string           `yaml:"skip_backfills,omitempty"`
	BackfillChunkSize        int              `yaml:"backfill_chunk_size,omitempty"`
	DiscoverSchemas          []string         `yaml:"discover_schemas,omitempty"`
	SourceTag                string           `yaml:"source_tag,omitempty"`
	RediscoveryInterval      string           `yaml:"rediscovery_interval,omitempty"`
	FeatureFlags             string           `yaml:"feature_flags,omitempty"`
	StatementTimeout         StatementTimeout `yaml:"statement_timeout,omitempty" enum:",30s,1m,5m,30m"`
	SSLMode                  sslmode.MySQL    `yaml:"sslmode,omitempty" enum:"disabled,preferred,required,verify_ca,verify_identity"`
	SSLServerCA              string           `yaml:"ssl_server_ca,omitempty" secret:"true"`
	SSLClientCert            string           `yaml:"ssl_client_cert,omitempty" secret:"true"`
	SSLClientKey             string           `yaml:"ssl_client_key,omitempty"`
	WatermarksTable          string           `yaml:"watermarks_table,omitempty"`
	HeartbeatInterval        string           `yaml:"heartbeat_interval,omitempty"`
}

// Config is the source-mysql configuration shared by every image in Images. Password is the deprecated form of
// Credentials.
type Config struct {
	Address          string                    `yaml:"address"`
	User             string                    `yaml:"user"`
	Password         string                    `yaml:"password,omitempty"`
	Credentials      *auth.DatabaseCredentials `yaml:"credentials,omitempty"`
	Timezone         string                    `yaml:"timezone,omitempty"`
	HistoryMode      bool                      `yaml:"historyMode"`
	DiscoveryFilters *DiscoveryFilters         `yaml:"discoveryFilters,omitempty"`
	Advanced         *Advanced                 `yaml:"advanced,omitempty"`
	NetworkTunnel    *endpoint.NetworkTunnel   `yaml:"networkTunnel,omitempty"`
}

// Resource identifies the captured table, shared by the SQL captures.
type Resource = sqlcapture.Resource

// Binding maps a captured MySQL resource to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full MySQL capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
