package mysql

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/sqlcapture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// SSLMode controls whether connections use TLS and how strictly the server certificate is verified.
type SSLMode string

// StatementTimeout overrides the connector statement timeout, blank disables it.
type StatementTimeout string

// Image is the Aurora MySQL connector image.
const Image string = "ghcr.io/estuary/source-amazon-aurora-mysql:v3"

// ConnectorName ends the catalog name of every task of the connector, as the dashboard names them.
const ConnectorName string = "source-amazon-aurora-mysql"

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
	StatementTimeout         StatementTimeout `yaml:"statement_timeout,omitempty"`
	SSLMode                  SSLMode          `yaml:"sslmode,omitempty"`
	SSLServerCA              string           `yaml:"ssl_server_ca,omitempty"`
	SSLClientCert            string           `yaml:"ssl_client_cert,omitempty"`
	SSLClientKey             string           `yaml:"ssl_client_key,omitempty"`
	WatermarksTable          string           `yaml:"watermarks_table,omitempty"`
	HeartbeatInterval        string           `yaml:"heartbeat_interval,omitempty"`
}

// Config carries the ghcr.io/estuary/source-amazon-aurora-mysql connector configurations, including
// the database connection, its authentication and the binlog replication settings. Password is
// the deprecated form of Credentials.
type Config struct {
	Address          string                    `yaml:"address"`
	User             string                    `yaml:"user"`
	Password         string                    `yaml:"password,omitempty"`
	Credentials      *auth.DatabaseCredentials `yaml:"credentials,omitempty"`
	Timezone         string                    `yaml:"timezone,omitempty"`
	HistoryMode      bool                      `yaml:"historyMode,omitempty"`
	DiscoveryFilters *DiscoveryFilters         `yaml:"discoveryFilters,omitempty"`
	Advanced         *Advanced                 `yaml:"advanced,omitempty"`
	NetworkTunnel    *endpoint.NetworkTunnel   `yaml:"networkTunnel,omitempty"`
}

// Resource identifies the captured table, shared by the SQL captures.
type Resource = sqlcapture.Resource

// Binding maps a captured Aurora MySQL resource to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Aurora MySQL capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
