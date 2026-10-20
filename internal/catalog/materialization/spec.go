package materialization

// DBTJobTriggerMode controls how a trigger behaves when the dbt job is already running.
type DBTJobTriggerMode string

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

// SyncSchedule controls how often the materialization commits transactions.
type SyncSchedule struct {
	SyncFrequency       string `yaml:"syncFrequency,omitempty"`
	Timezone            string `yaml:"timezone,omitempty"`
	FastSyncStartTime   string `yaml:"fastSyncStartTime,omitempty"`
	FastSyncStopTime    string `yaml:"fastSyncStopTime,omitempty"`
	FastSyncEnabledDays string `yaml:"fastSyncEnabledDays,omitempty"`
}

// DBTJobTrigger triggers a dbt Cloud job when the materialization commits new data.
type DBTJobTrigger struct {
	JobID     string            `yaml:"job_id"`
	AccountID string            `yaml:"account_id"`
	APIKey    string            `yaml:"api_key"`
	AccessURL string            `yaml:"access_url,omitempty"`
	Cause     string            `yaml:"cause,omitempty"`
	Mode      DBTJobTriggerMode `yaml:"mode,omitempty"`
	Interval  string            `yaml:"interval,omitempty"`
}

// Source describes the default source setting when set for the materialization, a nil Capture is
// emitted as null. TargetNaming controls the binding naming strategy and how they are generated
// from collection names.
type Source struct {
	Capture           *string `yaml:"capture"`
	TargetNaming      string  `yaml:"targetNaming"`
	DeltaUpdates      bool    `yaml:"deltaUpdates"`
	FieldsRecommended int     `yaml:"fieldsRecommended"`
}

// BindingFields controls which collection fields a binding materializes, Require always including its fields.
type BindingFields struct {
	Recommended bool                `yaml:"recommended"`
	Exclude     []string            `yaml:"exclude,omitempty"`
	Require     map[string]struct{} `yaml:"require,omitempty"`
}

// Binding maps a source collection to the materialized resource, R being the connector resource.
type Binding[R any] struct {
	Resource R              `yaml:"resource"`
	Source   string         `yaml:"source"`
	Fields   *BindingFields `yaml:"fields,omitempty"`
	Backfill int            `yaml:"backfill,omitempty"`
}

// Spec is a full materialization entry keyed by its catalog name under the materializations section of
// a catalog spec, E being the endpoint (e.g. endpoint.ConnectorSpec[Config]) and R being the connector resource.
type Spec[E, R any] struct {
	Endpoint E            `yaml:"endpoint"`
	Source   *Source      `yaml:"source,omitempty"`
	Bindings []Binding[R] `yaml:"bindings"`
}

// MaterializationSpec marks Spec as a materialization spec, satisfying catalog.MaterializationSpec.
func (Spec[E, R]) MaterializationSpec() {}

// GeneratesBindings reports whether a source capture may add bindings to the materialization, satisfying catalog.BindingsGenerator.
func (spec Spec[E, R]) GeneratesBindings() bool {
	return spec.Source != nil && spec.Source.Capture != nil
}
