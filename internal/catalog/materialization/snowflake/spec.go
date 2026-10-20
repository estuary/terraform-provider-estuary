package snowflake

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// AuthType identifies how the connector authenticates its Snowflake user, user_password being deprecated.
type AuthType string

// TimestampType controls how timestamp columns are stored in Snowflake.
type TimestampType string

// Image is the Snowflake connector image.
const Image string = "ghcr.io/estuary/materialize-snowflake:v4"

// ConnectorName ends the catalog name of every task of the connector, as the dashboard names them.
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

// Credentials carries the Snowflake authentication, with the fields relevant to its AuthType.
type Credentials struct {
	AuthType   AuthType `yaml:"auth_type"`
	User       string   `yaml:"user,omitempty"`
	PrivateKey string   `yaml:"private_key,omitempty"`
	Password   string   `yaml:"password,omitempty"`
}

// Advanced carries the advanced endpoint options, including field truncation and feature flags.
type Advanced struct {
	DisableFieldTruncation bool   `yaml:"disableFieldTruncation,omitempty"`
	NoFlowDocument         bool   `yaml:"no_flow_document,omitempty"`
	FeatureFlags           string `yaml:"feature_flags,omitempty"`
}

// Config carries the ghcr.io/estuary/materialize-snowflake connector configurations, including
// the account connection, its authentication and the transactions sync schedule.
type Config struct {
	Host          string                         `yaml:"host"`
	Database      string                         `yaml:"database"`
	Schema        string                         `yaml:"schema"`
	Warehouse     string                         `yaml:"warehouse,omitempty"`
	Role          string                         `yaml:"role,omitempty"`
	Account       string                         `yaml:"account,omitempty"`
	TimestampType TimestampType                  `yaml:"timestamp_type"`
	HardDelete    bool                           `yaml:"hardDelete,omitempty"`
	Credentials   Credentials                    `yaml:"credentials"`
	SyncSchedule  *materialization.SyncSchedule  `yaml:"syncSchedule,omitempty"`
	DBTJobTrigger *materialization.DBTJobTrigger `yaml:"dbt_job_trigger,omitempty"`
	Advanced      *Advanced                      `yaml:"advanced,omitempty"`
}

// Resource identifies the target Snowflake table, Schema overriding the endpoint schema.
type Resource struct {
	Table        string `yaml:"table"`
	Schema       string `yaml:"schema,omitempty"`
	DeltaUpdates bool   `yaml:"delta_updates,omitempty"`
	Clustering   string `yaml:"clustering,omitempty"`
}

// Binding maps a source collection to the Snowflake table it materializes into.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Snowflake materialization entry, keyed by its catalog name in a catalog spec.
type Spec = materialization.Spec[EndpointSpec, Resource]
