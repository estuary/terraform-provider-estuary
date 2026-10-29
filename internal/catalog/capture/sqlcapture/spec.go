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
	AdditionalBackfillFilter string `yaml:"additional_backfill_filter,omitempty" description:"SQL filter clause applied to every backfill query of the table, so the rows it excludes are never backfilled. It can't be used with the 'Precise' mode, and combines with config.advanced.min_backfill_xid where the connector has it. Setting or changing it requires a backfill of the binding. Contact Estuary support before using it."`
}

// Resource identifies the captured table of a SQL change data capture binding.
type Resource struct {
	Namespace  string            `yaml:"namespace" description:"Schema of the captured table, or its database on MySQL."`
	Stream     string            `yaml:"stream" description:"Name of the captured table."`
	Mode       BackfillMode      `yaml:"mode,omitempty" enum:",Normal,Precise,Only Changes,Without Primary Key" backfill:"mode" description:"How the table contents are backfilled. Leave it unset to let the connector choose: 'Normal' emits every change right away, 'Precise' holds back changes to rows the backfill hasn't reached yet, 'Only Changes' skips the backfill and only captures new changes, and 'Without Primary Key' backfills tables without a primary key. The connector requires a backfill along with a change, so raise trigger, or the capture's, in the same change: the plan fails otherwise."`
	Priority   int               `yaml:"priority,omitempty" description:"Backfill priority. Bindings with the highest priority are backfilled completely before the others. Defaults to 0, negative values backfill after the others."`
	Advanced   *ResourceAdvanced `yaml:"advanced,omitempty" description:"Rarely needed options of the binding."`
	PrimaryKey []string          `yaml:"primary_key,omitempty" description:"Columns used to order the backfill and merge changes, overriding the collection key. Rarely needed."`
}

// ResourcePath returns the namespace and stream, satisfying capture.ResourcePather.
func (resource Resource) ResourcePath() []string {
	return []string{resource.Namespace, resource.Stream}
}
