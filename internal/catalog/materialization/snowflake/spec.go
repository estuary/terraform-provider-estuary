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
	AuthType   AuthType `yaml:"auth_type" enum:"jwt,user_password"`
	User       string   `yaml:"user,omitempty"`
	PrivateKey string   `yaml:"private_key,omitempty"`
	Password   string   `yaml:"password,omitempty"`
}

// Advanced holds advanced endpoint options, such as field truncation.
type Advanced struct {
	DisableFieldTruncation bool   `yaml:"disableFieldTruncation,omitempty"`
	NoFlowDocument         bool   `yaml:"no_flow_document,omitempty"`
	FeatureFlags           string `yaml:"feature_flags,omitempty"`
}

// Config is the materialize-snowflake connector configuration.
type Config struct {
	Host          string                         `yaml:"host"`
	Database      string                         `yaml:"database"`
	Schema        string                         `yaml:"schema"`
	Warehouse     string                         `yaml:"warehouse,omitempty"`
	Role          string                         `yaml:"role,omitempty"`
	Account       string                         `yaml:"account,omitempty"`
	TimestampType TimestampType                  `yaml:"timestamp_type" enum:"TIMESTAMP_LTZ,TIMESTAMP_TZ,TIMESTAMP_NTZ (discard TZ),TIMESTAMP_NTZ (normalize to UTC)"`
	HardDelete    bool                           `yaml:"hardDelete,omitempty"`
	Credentials   Credentials                    `yaml:"credentials"`
	SyncSchedule  *materialization.SyncSchedule  `yaml:"syncSchedule,omitempty"`
	DBTJobTrigger *materialization.DBTJobTrigger `yaml:"dbt_job_trigger,omitempty"`
	Advanced      *Advanced                      `yaml:"advanced,omitempty"`
}

// Resource identifies the target Snowflake table; Schema overrides the endpoint schema.
type Resource struct {
	Table        string `yaml:"table"`
	Schema       string `yaml:"schema,omitempty"`
	DeltaUpdates bool   `yaml:"delta_updates,omitempty"`
	Clustering   string `yaml:"clustering,omitempty"`
}

// Binding maps a source collection to its Snowflake table.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a Snowflake materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
