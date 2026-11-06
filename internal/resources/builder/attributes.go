package builder

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Entry attributes.
const (
	nameAttribute                  string = "name"
	catalogNameAttribute           string = "catalog_name"
	imageAttribute                 string = "image"
	disableAttribute               string = "disable"
	shardsAttribute                string = "shards"
	bindingsAttribute              string = "bindings"
	liveBindingsAttribute          string = "live_bindings"
	keepUnmanagedBindingsAttribute string = "keep_unmanaged_bindings"
	configAttribute                string = "config"
	targetNamingAttribute          string = "target_naming"
)

// Binding attributes.
const (
	bindingResourceAttribute string = "resource"
	bindingTargetAttribute   string = "target"
	bindingSourceAttribute   string = "source"
	bindingNameAttribute     string = "name"
	bindingFieldsAttribute   string = "fields"
	resourcePathAttribute    string = "resource_path"
)

// resourcePathType is the type of a binding resource_path.
var resourcePathType tftypes.List = tftypes.List{ElementType: tftypes.String}

// Binding fields attributes.
const (
	bindingFieldsRecommendedAttribute string = "recommended"
)

// Backfill block attributes, of the entry and of each binding.
const (
	backfillAttribute         string = "backfill"
	backfillTriggerAttribute  string = "trigger"
	backfillStrategyAttribute string = "strategy"
	backfillModeAttribute     string = "mode"
)

// Paths of the entry attributes read by path.
var (
	catalogNamePath           *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(catalogNameAttribute)
	shardsPath                *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(shardsAttribute)
	backfillPath              *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(backfillAttribute)
	bindingsPath              *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(bindingsAttribute)
	liveBindingsPath          *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(liveBindingsAttribute)
	keepUnmanagedBindingsPath *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(keepUnmanagedBindingsAttribute)
	configPath                *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(configAttribute)
	targetNamingPath          *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(targetNamingAttribute)
)

// withAttribute returns value with the attribute at path replaced by what replace returns for it, unchanged when value
// doesn't hold it.
func withAttribute(value tftypes.Value, path *tftypes.AttributePath, replace func(current tftypes.Value) tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(value, func(attributePath *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		if !attributePath.Equal(path) {
			return current, nil
		}

		return replace(current), nil
	})
}

// exists reports whether an entry value holds an entry, false for the null or untyped state of one not created yet.
func exists(entry tftypes.Value) bool {
	return entry.Type() != nil && !entry.IsNull()
}

// known converts a value, reporting false when it is null, unknown or of another type.
func known[T any](value tftypes.Value) (T, bool) {
	var converted T

	if !value.IsKnown() || value.IsNull() || value.As(&converted) != nil {
		return converted, false
	}

	return converted, true
}

// blockAttribute returns an attribute of an object value, null when absent and unknown when the object is.
func blockAttribute(block tftypes.Value, attribute string, attributeType tftypes.Type) tftypes.Value {
	var attributes map[string]tftypes.Value

	if !block.IsKnown() {
		return tftypes.NewValue(attributeType, tftypes.UnknownValue)
	}

	if block.IsNull() || block.As(&attributes) != nil {
		return tftypes.NewValue(attributeType, nil)
	}

	if value, found := attributes[attribute]; found {
		return value
	}

	return tftypes.NewValue(attributeType, nil)
}
