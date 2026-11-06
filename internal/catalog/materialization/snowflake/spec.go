package snowflake

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// AuthType is how the connector authenticates its Snowflake user; user_password is deprecated.
type AuthType string

// TimestampType is how timestamp columns are stored in Snowflake.
type TimestampType string

// Image is the Snowflake connector image.
const Image string = "ghcr.io/estuary/materialize-snowflake:v4"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "materialize-snowflake"

const (
	AuthTypeJWT          AuthType = "jwt"
	AuthTypeUserPassword AuthType = "user_password"
)

// AuthTypes lists every supported auth type, useful for schema validators.
var AuthTypes = []AuthType{
	AuthTypeJWT,
	AuthTypeUserPassword,
}

const (
	TimestampTypeLTZ          TimestampType = "TIMESTAMP_LTZ"
	TimestampTypeNTZDiscard   TimestampType = "TIMESTAMP_NTZ (discard TZ)"
	TimestampTypeNTZNormalize TimestampType = "TIMESTAMP_NTZ (normalize to UTC)"
	TimestampTypeTZ           TimestampType = "TIMESTAMP_TZ"
)

// TimestampTypes lists every supported timestamp type, useful for schema validators.
var TimestampTypes = []TimestampType{
	TimestampTypeLTZ,
	TimestampTypeNTZDiscard,
	TimestampTypeNTZNormalize,
	TimestampTypeTZ,
}

// Credentials is the Snowflake authentication, using the fields of its AuthType.
type Credentials struct {
	AuthType   AuthType `yaml:"auth_type" enum:"jwt,user_password" description:"Authentication method: 'jwt' (key-pair) or the deprecated 'user_password'. Only the attributes of the selected method are used."`
	User       string   `yaml:"user" description:"Snowflake user login name."`
	PrivateKey string   `yaml:"private_key,omitempty" description:"PEM-encoded PKCS#8 private key used to sign the JWT, with 'jwt'. Write-only."`
	Password   string   `yaml:"password,omitempty" description:"Password of user, with the deprecated 'user_password'. Write-only."`
}

// Advanced holds advanced endpoint options, such as field truncation.
type Advanced struct {
	DisableFieldTruncation bool   `yaml:"disableFieldTruncation,omitempty" description:"Disables truncation of materialized fields. May result in errors for documents with extremely large values or complex nested structures."`
	NoFlowDocument         bool   `yaml:"no_flow_document,omitempty" description:"Stops requiring the root document for standard updates. Defaults to false."`
	FeatureFlags           string `yaml:"feature_flags,omitempty" description:"Connector settings for edge cases and experimental features, see https://docs.estuary.dev/guides/advanced-usage/feature-flags/. Intended for Estuary internal use: change them only as directed by Estuary support."`
}

// Config is the materialize-snowflake connector configuration.
type Config struct {
	Host          string                         `yaml:"host" credentials:"forbidden" description:"Snowflake account URL, without the protocol and ending in .snowflakecomputing.com (e.g., 'orgname-accountname.snowflakecomputing.com'). The plan rejects credentials in it, such as user:password@ or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as ***."`
	Database      string                         `yaml:"database" description:"Snowflake database to materialize into."`
	Schema        string                         `yaml:"schema" x-default-namespace:"true" description:"Schema of the bound tables, unless a binding sets its own, and of the materialization metadata tables."`
	Warehouse     string                         `yaml:"warehouse,omitempty" description:"Snowflake virtual warehouse that runs the queries. Defaults to the user's default warehouse."`
	Role          string                         `yaml:"role,omitempty" description:"Role the user performs actions with."`
	Account       string                         `yaml:"account,omitempty" description:"Snowflake account identifier. Optional and hidden in the dashboard."`
	TimestampType TimestampType                  `yaml:"timestamp_type" enum:"TIMESTAMP_LTZ,TIMESTAMP_TZ,TIMESTAMP_NTZ (discard TZ),TIMESTAMP_NTZ (normalize to UTC)" description:"How timestamp columns are stored: 'TIMESTAMP_LTZ', 'TIMESTAMP_TZ', 'TIMESTAMP_NTZ (discard TZ)' or 'TIMESTAMP_NTZ (normalize to UTC)'. See https://docs.estuary.dev/reference/Connectors/materialization-connectors/Snowflake/#timestamp-data-type-mapping."`
	HardDelete    bool                           `yaml:"hardDelete,omitempty" description:"Deletes rows from the destination when they're deleted in the source. Otherwise _meta/op marks deleted rows (soft delete). Defaults to false."`
	Credentials   Credentials                    `yaml:"credentials" description:"How the connector authenticates with Snowflake."`
	SyncSchedule  *materialization.SyncSchedule  `yaml:"syncSchedule,omitempty" description:"Schedule of the materialization transactions."`
	DBTJobTrigger *materialization.DBTJobTrigger `yaml:"dbt_job_trigger,omitempty" description:"dbt Cloud job triggered when new data is available."`
	Advanced      *Advanced                      `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
}

// Resource identifies the target Snowflake table; Schema overrides the endpoint schema.
type Resource struct {
	Table        string `yaml:"table" x-collection-name:"true" description:"Snowflake table to materialize into."`
	Schema       string `yaml:"schema,omitempty" x-schema-name:"true" description:"Schema of the table, overriding config.schema."`
	DeltaUpdates bool   `yaml:"delta_updates,omitempty" x-delta-updates:"true" description:"Appends the changes of each transaction instead of merging them into one row per key. With 'jwt' credentials, delta updates bindings load through Snowpipe Streaming. Defaults to false. Switching it from true to false requires a backfill: in the same change, set or raise backfill.trigger, or set or raise the materialization's, which backfills every enabled binding, or the plan fails."`
	Clustering   string `yaml:"clustering,omitempty" description:"Comma-separated materialized column names used as the clustering key for Snowflake Automatic Clustering (e.g., 'date, user_id'); expressions aren't accepted. It's applied each time the materialization restarts, is published or backfills. Empty drops the table's clustering key, one set outside Estuary included. Recommended only for large tables queried with selective filters on these columns, as it consumes additional Snowflake credits."`
}

// Binding maps a source collection to its Snowflake table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a Snowflake materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
