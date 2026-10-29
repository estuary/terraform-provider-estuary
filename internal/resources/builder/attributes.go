package builder

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const nameAttribute string = "name"
const imageAttribute string = "image"
const disableAttribute string = "disable"
const shardsAttribute string = "shards"

const bindingsAttribute string = "bindings"
const bindingResourceAttribute string = "resource"
const bindingTargetAttribute string = "target"
const bindingSourceAttribute string = "source"
const bindingNameAttribute string = "name"
const liveBindingsAttribute string = "live_bindings"

const backfillAttribute string = "backfill"
const backfillTriggerAttribute string = "trigger"
const backfillStrategyAttribute string = "strategy"
const backfillModeAttribute string = "mode"

// bindingsPath is the path of the bindings attribute.
var bindingsPath *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(bindingsAttribute)

// liveBindingsPath is the path of the live_bindings attribute.
var liveBindingsPath *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(liveBindingsAttribute)

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
