package builder

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Entry attributes.
const (
	nameAttribute                  string = "name"
	imageAttribute                 string = "image"
	disableAttribute               string = "disable"
	shardsAttribute                string = "shards"
	bindingsAttribute              string = "bindings"
	liveBindingsAttribute          string = "live_bindings"
	keepUnmanagedBindingsAttribute string = "keep_unmanaged_bindings"
)

// Binding attributes.
const (
	bindingResourceAttribute string = "resource"
	bindingTargetAttribute   string = "target"
	bindingSourceAttribute   string = "source"
	bindingNameAttribute     string = "name"
	bindingFieldsAttribute   string = "fields"
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
	bindingsPath              *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(bindingsAttribute)
	liveBindingsPath          *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(liveBindingsAttribute)
	keepUnmanagedBindingsPath *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(keepUnmanagedBindingsAttribute)
)

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
