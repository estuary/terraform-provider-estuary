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
var bindingSourceKeys []string = []string{"name", "notBefore", "notAfter"}

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
	JobID     string            `yaml:"job_id" description:"ID of the dbt job to trigger."`
	AccountID string            `yaml:"account_id" description:"ID of the dbt account."`
	APIKey    string            `yaml:"api_key" description:"dbt API key used to trigger the job. Write-only."`
	AccessURL string            `yaml:"access_url,omitempty" description:"Access URL of the dbt account, found in its Account Settings (e.g., https://cloud.getdbt.com). It must start with https://. The job is only triggered when it's set."`
	Cause     string            `yaml:"cause,omitempty" description:"Message shown as the cause of each triggered run. Defaults to 'Estuary'."`
	Mode      DBTJobTriggerMode `yaml:"mode,omitempty" enum:"replace,ignore,skip" description:"What happens when the job is already running: 'skip' doesn't trigger it (default), 'replace' cancels the running job and triggers a new one, and 'ignore' triggers a new one anyway."`
	Interval  string            `yaml:"interval,omitempty" description:"Minimum time between two triggers, which only happen after new data is materialized. Defaults to '30m'."`
}

// Source sets how added bindings are initialized; a non-nil Capture binds each of its collections, nil emits null.
type Source struct {
	Capture           *string    `yaml:"capture" description:"Full name of a capture whose bindings the materialization follows. The provider tenant isn't added to it. Each collection added to the capture gets a binding here, and bindings removed from the capture are kept."`
	DeltaUpdates      bool       `yaml:"deltaUpdates,omitempty" description:"Whether added bindings use delta updates. Defaults to false."`
	FieldsRecommended FieldDepth `yaml:"fieldsRecommended,omitempty" description:"fields.recommended of added bindings: '0', '1', '2', ... or 'unlimited', see bindings.fields. Defaults to 'unlimited'."`
}

// TargetNaming sets how added bindings name their schema and table; Schema applies to single schema strategies.
type TargetNaming struct {
	Strategy           TargetNamingStrategy `yaml:"strategy" enum:"matchSourceStructure,singleSchema,prefixTableNames" description:"How added bindings name their schema and table from their collection: 'matchSourceStructure' keeps the source schema and table, 'singleSchema' puts every table in schema, and 'prefixTableNames' puts every table in schema with the source schema as table prefix."`
	Schema             string               `yaml:"schema,omitempty" description:"Schema of every table, with 'singleSchema' and 'prefixTableNames'."`
	SkipCommonDefaults bool                 `yaml:"skipCommonDefaults,omitempty" description:"With 'prefixTableNames', doesn't prefix the common default schemas 'public' and 'dbo'. Defaults to false."`
	SchemaTemplate     string               `yaml:"schemaTemplate,omitempty" description:"With 'matchSourceStructure', template of the schema name around {{schema}} (e.g., 'raw_{{schema}}')."`
	TableTemplate      string               `yaml:"tableTemplate,omitempty" description:"Template of the table name around {{table}} (e.g., '{{table}}_v2')."`
}

// BindingFields selects fields up to Recommended depth, plus Require, minus Exclude; GroupBy replaces the key.
type BindingFields struct {
	Recommended FieldDepth          `yaml:"recommended" description:"Depth of the fields selected by default: '0' selects only the root document and the required fields, such as the collection key, '1' the top-level fields, '2' the second-level fields too, and so on, and 'unlimited' every field."`
	Exclude     []string            `yaml:"exclude,omitempty" description:"Fields never materialized."`
	Require     map[string]struct{} `yaml:"require,omitempty" description:"Fields always materialized, whatever recommended selects."`
	GroupBy     []string            `yaml:"groupBy,omitempty" description:"Fields keying the table instead of the collection key. Only fields with a scalar type (boolean, integer or string) can be keys, and changing them backfills the binding."`
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
	Source                     *Source                    `yaml:"source,omitempty" description:"Settings of the bindings Estuary adds, optionally following a capture."`
	TargetNaming               *TargetNaming              `yaml:"targetNaming,omitempty" description:"How Estuary names the tables of the bindings it adds."`
	OnIncompatibleSchemaChange OnIncompatibleSchemaChange `yaml:"onIncompatibleSchemaChange,omitempty" enum:"backfill,disableBinding,disableTask,abort" description:"What an automated change, such as a capture discovery, does when it's incompatible with a materialized table: 'backfill' (default) backfills the affected bindings, 'disableBinding' disables them, 'disableTask' disables the materialization, and 'abort' stops the automated change. Bindings can override it."`
	Shards                     catalog.Shards             `yaml:"shards,omitempty" description:"Shard settings of the materialization, with Estuary's own keys such as logLevel and disable."`
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

// AddsNewBindings reports whether a source capture binds each of its collections, satisfying catalog.NewBindingsAdder.
func (spec Spec[E, R]) AddsNewBindings() bool {
	return spec.GeneratesBindings()
}
