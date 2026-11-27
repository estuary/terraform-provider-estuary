package databricks

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// AuthType is how the connector authenticates with the Databricks SQL warehouse.
type AuthType string

// Image is the Databricks connector image.
const Image string = "ghcr.io/estuary/materialize-databricks:v3"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "materialize-databricks"

const (
	AuthTypePAT       AuthType = "PAT"
	AuthTypeOAuth2M2M AuthType = "OAuth2 M2M"
)

// AuthTypes lists every supported auth type, useful for schema validators.
var AuthTypes = []AuthType{
	AuthTypePAT,
	AuthTypeOAuth2M2M,
}

// Credentials is the Databricks authentication, using the fields of its AuthType.
type Credentials struct {
	AuthType            AuthType `yaml:"auth_type" enum:"PAT,OAuth2 M2M" description:"Authentication method: 'PAT' (personal access token) or 'OAuth2 M2M' (the client ID and secret of a service principal). Only the attributes of the selected method are used."`
	PersonalAccessToken string   `yaml:"personal_access_token,omitempty" description:"Personal access token with access to the SQL warehouse, with 'PAT'. Write-only."`
	ClientID            string   `yaml:"client_id,omitempty" secret:"true" description:"OAuth2 client ID of the Databricks service principal, with 'OAuth2 M2M'. Write-only."`
	ClientSecret        string   `yaml:"client_secret,omitempty" description:"OAuth2 client secret of the Databricks service principal, with 'OAuth2 M2M'. Write-only."`
}

// Advanced holds the advanced endpoint options.
type Advanced struct {
	NoFlowDocument bool   `yaml:"no_flow_document,omitempty" description:"Stops requiring the root document for standard updates. Defaults to false."`
	FeatureFlags   string `yaml:"feature_flags,omitempty" description:"Connector settings for edge cases and experimental features, see https://docs.estuary.dev/guides/advanced-usage/feature-flags/. Intended for Estuary internal use: change them only as directed by Estuary support."`
}

// Config is the materialize-databricks connector configuration.
type Config struct {
	Address       string                         `yaml:"address" credentials:"forbidden" description:"Host of the SQL warehouse, as host[:port] (e.g., 'dbc-abcdef12-3456.cloud.databricks.com'). The port defaults to 443. The plan rejects credentials in it, such as user:password@ or a password, token, API key or signature option, as state and logs would hold them in plain text. Credentials found in the live value show as ***."`
	HTTPPath      string                         `yaml:"http_path" description:"HTTP path of the SQL warehouse (e.g., '/sql/1.0/warehouses/abcdef1234567890'), from its connection details."`
	CatalogName   string                         `yaml:"catalog_name" description:"Unity Catalog to materialize into."`
	SchemaName    string                         `yaml:"schema_name" x-default-namespace:"true" description:"Schema of the bound tables, unless a binding sets its own, and of the materialization metadata tables. The connector's own default is 'default'."`
	HardDelete    bool                           `yaml:"hardDelete,omitempty" description:"Deletes rows from the destination when they're deleted in the source. Otherwise _meta/op marks deleted rows (soft delete). Defaults to false."`
	Credentials   Credentials                    `yaml:"credentials" description:"How the connector authenticates with Databricks."`
	SyncSchedule  *materialization.SyncSchedule  `yaml:"syncSchedule,omitempty" description:"Schedule of the materialization transactions."`
	DBTJobTrigger *materialization.DBTJobTrigger `yaml:"dbt_job_trigger,omitempty" description:"dbt Cloud job triggered when new data is available."`
	Advanced      *Advanced                      `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
}

// Resource identifies the target Databricks table; Schema overrides the endpoint schema.
type Resource struct {
	Table                    string `yaml:"table" x-collection-name:"true" description:"Databricks table to materialize into."`
	Schema                   string `yaml:"schema,omitempty" x-schema-name:"true" description:"Schema of the table, overriding config.schema_name."`
	DeltaUpdates             bool   `yaml:"delta_updates,omitempty" x-delta-updates:"true" description:"Appends the changes of each transaction instead of merging them into one row per key. Defaults to false. Switching it from true to false requires a backfill: in the same change, set or raise backfill.trigger, or set or raise the materialization's, which backfills every enabled binding, or the plan fails."`
	AdditionalTableCreateSQL string `yaml:"additional_table_create_sql,omitempty" description:"SQL statements the connector runs after it creates the table, such as an ALTER TABLE setting table properties. Requires an image tag of v3 or later."`
}

// Binding maps a source collection to its Databricks table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a Databricks materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
