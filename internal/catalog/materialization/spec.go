package materialization

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"gopkg.in/yaml.v3"
)

// DBTJobTriggerMode is how a trigger behaves while the dbt job is running.
type DBTJobTriggerMode string

// FieldDepth is the default field selection depth: "0" for required fields only, "unlimited" for all.
type FieldDepth string

// OnIncompatibleSchemaChange is the action taken when an automated change is incompatible with the resource.
type OnIncompatibleSchemaChange string

// TargetNamingStrategy is how added bindings name their destination schema and table.
type TargetNamingStrategy string

const (
	DBTJobTriggerModeSkip    DBTJobTriggerMode = "skip"
	DBTJobTriggerModeReplace DBTJobTriggerMode = "replace"
	DBTJobTriggerModeIgnore  DBTJobTriggerMode = "ignore"
)

// DBTJobTriggerModes lists every supported trigger mode, useful for schema validators.
var DBTJobTriggerModes = []DBTJobTriggerMode{
	DBTJobTriggerModeSkip,
	DBTJobTriggerModeReplace,
	DBTJobTriggerModeIgnore,
}

const FieldDepthUnlimited FieldDepth = "unlimited"

const (
	OnIncompatibleSchemaChangeBackfill       OnIncompatibleSchemaChange = "backfill"
	OnIncompatibleSchemaChangeDisableBinding OnIncompatibleSchemaChange = "disableBinding"
	OnIncompatibleSchemaChangeDisableTask    OnIncompatibleSchemaChange = "disableTask"
	OnIncompatibleSchemaChangeAbort          OnIncompatibleSchemaChange = "abort"
)

// OnIncompatibleSchemaChanges lists every supported action, useful for schema validators.
var OnIncompatibleSchemaChanges = []OnIncompatibleSchemaChange{
	OnIncompatibleSchemaChangeBackfill,
	OnIncompatibleSchemaChangeDisableBinding,
	OnIncompatibleSchemaChangeDisableTask,
	OnIncompatibleSchemaChangeAbort,
}

const (
	TargetNamingMatchSourceStructure TargetNamingStrategy = "matchSourceStructure"
	TargetNamingSingleSchema         TargetNamingStrategy = "singleSchema"
	TargetNamingPrefixTableNames     TargetNamingStrategy = "prefixTableNames"
)

// TargetNamingStrategies lists every supported target naming strategy, useful for schema validators.
var TargetNamingStrategies = []TargetNamingStrategy{
	TargetNamingMatchSourceStructure,
	TargetNamingSingleSchema,
	TargetNamingPrefixTableNames,
}

// bindingSourceKeys are the keys of a binding source object.
var bindingSourceKeys = []string{"name", "notBefore", "notAfter"}

// SyncSchedule sets how often the materialization commits transactions.
type SyncSchedule struct {
	SyncFrequency       string `yaml:"syncFrequency,omitempty" enum:"0s,30s,1m,2m30s,5m,15m,20m,30m,1h,2h,4h"`
	Timezone            string `yaml:"timezone,omitempty"`
	FastSyncStartTime   string `yaml:"fastSyncStartTime,omitempty"`
	FastSyncStopTime    string `yaml:"fastSyncStopTime,omitempty"`
	FastSyncEnabledDays string `yaml:"fastSyncEnabledDays,omitempty"`
}

// DBTJobTrigger runs a dbt Cloud job when the materialization commits new data.
type DBTJobTrigger struct {
	JobID     string            `yaml:"job_id"`
	AccountID string            `yaml:"account_id"`
	APIKey    string            `yaml:"api_key"`
	AccessURL string            `yaml:"access_url,omitempty"`
	Cause     string            `yaml:"cause,omitempty"`
	Mode      DBTJobTriggerMode `yaml:"mode,omitempty" enum:"replace,ignore,skip"`
	Interval  string            `yaml:"interval,omitempty"`
}

// Source sets how added bindings are initialized; a non-nil Capture binds each of its collections, nil emits null.
type Source struct {
	Capture           *string    `yaml:"capture"`
	DeltaUpdates      bool       `yaml:"deltaUpdates,omitempty"`
	FieldsRecommended FieldDepth `yaml:"fieldsRecommended,omitempty"`
}

// TargetNaming sets how added bindings name their schema and table; Schema applies to single schema strategies.
type TargetNaming struct {
	Strategy           TargetNamingStrategy `yaml:"strategy" enum:"matchSourceStructure,singleSchema,prefixTableNames"`
	Schema             string               `yaml:"schema,omitempty"`
	SkipCommonDefaults bool                 `yaml:"skipCommonDefaults,omitempty"`
	SchemaTemplate     string               `yaml:"schemaTemplate,omitempty"`
	TableTemplate      string               `yaml:"tableTemplate,omitempty"`
}

// BindingFields selects fields up to Recommended depth, plus Require, minus Exclude; GroupBy replaces the key.
type BindingFields struct {
	Recommended FieldDepth          `yaml:"recommended"`
	Exclude     []string            `yaml:"exclude,omitempty"`
	Require     map[string]struct{} `yaml:"require,omitempty"`
	GroupBy     []string            `yaml:"groupBy,omitempty"`
}

// BindingSource is the collection a binding reads, optionally filtered by publication time.
// Specs write it as a plain name when unfiltered.
type BindingSource struct {
	Name      string `yaml:"name"`
	NotBefore string `yaml:"notBefore,omitempty"`
	NotAfter  string `yaml:"notAfter,omitempty"`
}

// BindingOptions holds binding settings other than resource and source.
type BindingOptions struct {
	Disable                    bool                       `yaml:"disable,omitempty"`
	Priority                   int                        `yaml:"priority,omitempty"`
	Fields                     *BindingFields             `yaml:"fields,omitempty"`
	Backfill                   int                        `yaml:"backfill,omitempty"`
	OnIncompatibleSchemaChange OnIncompatibleSchemaChange `yaml:"onIncompatibleSchemaChange,omitempty" enum:"backfill,disableBinding,disableTask,abort"`
}

// Binding maps a source collection to the connector resource R.
type Binding[R any] struct {
	Resource       R             `yaml:"resource"`
	Source         BindingSource `yaml:"source"`
	BindingOptions `yaml:",inline"`
}

// SpecOptions holds materialization settings other than endpoint and bindings.
type SpecOptions struct {
	Source                     *Source                    `yaml:"source,omitempty"`
	TargetNaming               *TargetNaming              `yaml:"targetNaming,omitempty"`
	OnIncompatibleSchemaChange OnIncompatibleSchemaChange `yaml:"onIncompatibleSchemaChange,omitempty" enum:"backfill,disableBinding,disableTask,abort"`
	Shards                     catalog.Shards             `yaml:"shards,omitempty"`
}

// Spec is a materialization entry with endpoint E (e.g. endpoint.ConnectorSpec[Config]) and resource R.
type Spec[E, R any] struct {
	Endpoint    E `yaml:"endpoint"`
	SpecOptions `yaml:",inline"`
	Bindings    []Binding[R] `yaml:"bindings"`
}

// MarshalYAML writes a number, or true when unlimited.
func (depth FieldDepth) MarshalYAML() (any, error) {
	if depth == FieldDepthUnlimited {
		return true, nil
	}

	value, err := strconv.Atoi(string(depth))

	if err != nil || value < 0 {
		return nil, fmt.Errorf("field depth %q is neither a non-negative number nor %s", depth, FieldDepthUnlimited)
	}

	return value, nil
}

// UnmarshalYAML reads a number, true (unlimited), false (0) or the depth text.
func (depth *FieldDepth) UnmarshalYAML(node *yaml.Node) error {
	var flag bool

	if node.Tag == "!!bool" {
		if err := node.Decode(&flag); err != nil {
			return err
		}

		*depth = FieldDepth("0")

		if flag {
			*depth = FieldDepthUnlimited
		}

		return nil
	}

	if node.Value == string(FieldDepthUnlimited) {
		*depth = FieldDepthUnlimited
		return nil
	}

	value, err := strconv.Atoi(node.Value)

	if err != nil || value < 0 {
		return fmt.Errorf("field depth %q is neither a non-negative number nor %s", node.Value, FieldDepthUnlimited)
	}

	*depth = FieldDepth(strconv.Itoa(value))

	return nil
}

// MarshalYAML writes the collection name alone when unfiltered.
func (source BindingSource) MarshalYAML() (any, error) {
	type plain BindingSource

	if source.NotBefore == "" && source.NotAfter == "" {
		return source.Name, nil
	}

	return plain(source), nil
}

// UnmarshalYAML reads a collection name or an object, rejecting unknown keys.
func (source *BindingSource) UnmarshalYAML(node *yaml.Node) error {
	type plain BindingSource

	if node.Kind == yaml.ScalarNode {
		return node.Decode(&source.Name)
	}

	for index := 0; index+1 < len(node.Content); index += 2 {
		if key := node.Content[index].Value; !slices.Contains(bindingSourceKeys, key) {
			return fmt.Errorf("line %d: field %s not found in type materialization.BindingSource", node.Content[index].Line, key)
		}
	}

	return node.Decode((*plain)(source))
}

// MaterializationSpec satisfies catalog.MaterializationSpec.
func (Spec[E, R]) MaterializationSpec() {}

// GeneratesBindings reports whether a source capture may add bindings, satisfying catalog.BindingsGenerator.
func (spec Spec[E, R]) GeneratesBindings() bool {
	return spec.Source != nil && spec.Source.Capture != nil
}
