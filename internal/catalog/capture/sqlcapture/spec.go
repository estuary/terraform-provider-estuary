package sqlcapture

import (
	"regexp"
	"strings"
)

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
	Mode       BackfillMode      `yaml:"mode,omitempty"`
	Priority   int               `yaml:"priority,omitempty"`
	Advanced   *ResourceAdvanced `yaml:"advanced,omitempty"`
	PrimaryKey []string          `yaml:"primary_key,omitempty"`
}

// catalogNameSanitizer matches the characters collection names don't allow, as the connectors sanitize them.
var catalogNameSanitizer *regexp.Regexp = regexp.MustCompile(`(?i)[^a-z0-9\-_.]`)

// RecommendedName returns namespace/stream lowercased and sanitized, the collection name the connector discovery recommends.
func (resource Resource) RecommendedName() string {
	return catalogNameComponent(resource.Namespace) + "/" + catalogNameComponent(resource.Stream)
}

func catalogNameComponent(name string) string {
	name = catalogNameSanitizer.ReplaceAllString(strings.ToLower(name), "_")

	if name == "." || name == ".." {
		return strings.ReplaceAll(name, ".", "_")
	}

	return name
}
