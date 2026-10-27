package sqlcapture

// BackfillMode controls how the preexisting contents of a captured table are backfilled.
type BackfillMode string

const (
	BackfillModeAutomatic   BackfillMode = ""
	BackfillModeNormal      BackfillMode = "Normal"
	BackfillModePrecise     BackfillMode = "Precise"
	BackfillModeOnlyChanges BackfillMode = "Only Changes"
	BackfillModeWithoutKey  BackfillMode = "Without Primary Key"
)

// BackfillModes lists every supported backfill mode, useful for schema validators.
var BackfillModes = []BackfillMode{
	BackfillModeAutomatic,
	BackfillModeNormal,
	BackfillModePrecise,
	BackfillModeOnlyChanges,
	BackfillModeWithoutKey,
}

// ResourceAdvanced carries the rarely used per-binding options.
type ResourceAdvanced struct {
	AdditionalBackfillFilter string `yaml:"additional_backfill_filter,omitempty"`
}

// Resource identifies the captured table of a SQL change data capture binding.
type Resource struct {
	Namespace  string            `yaml:"namespace"`
	Stream     string            `yaml:"stream"`
	Mode       BackfillMode      `yaml:"mode,omitempty" enum:",Normal,Precise,Only Changes,Without Primary Key" backfill:"mode"`
	Priority   int               `yaml:"priority,omitempty"`
	Advanced   *ResourceAdvanced `yaml:"advanced,omitempty"`
	PrimaryKey []string          `yaml:"primary_key,omitempty"`
}

// ResourcePath returns the namespace and stream, satisfying capture.ResourcePather.
func (resource Resource) ResourcePath() []string {
	return []string{resource.Namespace, resource.Stream}
}
