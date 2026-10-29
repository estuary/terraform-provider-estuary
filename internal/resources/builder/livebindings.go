package builder

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// liveBindingsDescription describes the live_bindings attribute of resources and data sources.
const liveBindingsDescription string = "Every live binding, the declared ones and those Estuary adds, with the attributes of bindings."

// liveBindingsSchema returns the read-only list of every live binding, with the type of bindings.
func liveBindingsSchema(bindings schema.ListNestedAttribute) schema.ListAttribute {
	return schema.ListAttribute{
		Description: liveBindingsDescription + " Read-only: refreshed on each plan, and planned with the binding changes the apply publishes.",
		Computed:    true,
		ElementType: bindings.NestedObject.Type(),
	}
}

// withLiveBindings sets the live bindings of value to the bindings of live, empty when it has none. Values without
// live bindings are returned unchanged.
func withLiveBindings(value tftypes.Value, live tftypes.Value) (tftypes.Value, error) {
	items, _, err := bindings(live)

	if err != nil {
		return value, err
	}

	if items == nil {
		items = []tftypes.Value{}
	}

	return setLiveBindings(value, func(current tftypes.Value) tftypes.Value {
		return tftypes.NewValue(current.Type(), items)
	})
}

// setLiveBindings replaces the live bindings of value, leaving values without them unchanged.
func setLiveBindings(value tftypes.Value, replace func(current tftypes.Value) tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(value, func(attributePath *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		if !attributePath.Equal(liveBindingsPath) {
			return current, nil
		}

		return replace(current), nil
	})
}

// planLiveBindings plans the stored live bindings while bindings are unchanged, else merges the planned ones over them.
// Unknown on create with discovery or while unknown; documents without live bindings are returned unchanged.
func (codec *Codec[D, S]) planLiveBindings(planned tftypes.Value, state tftypes.Value) (tftypes.Value, error) {
	var created bool = state.Type() == nil || state.IsNull()
	var stored []tftypes.Value

	if !codec.catalogType.listsLiveBindings {
		return planned, nil
	}

	storedValue, storedErr := attributeValue(state, liveBindingsPath)

	if !created && storedErr == nil && storedValue.IsKnown() && sameBindings(planned, state) {
		return setLiveBindings(planned, func(tftypes.Value) tftypes.Value {
			return storedValue
		})
	}

	predicted, isKnown := codec.publishedBindings(planned, created)

	if !created {
		previous, _, err := bindings(state)

		isKnown = isKnown && err == nil && storedErr == nil && storedValue.IsKnown() && !storedValue.IsNull() && storedValue.As(&stored) == nil

		predicted = mergeBindings(predicted, previous, stored, bindingCollectionValue)
	}

	return setLiveBindings(planned, func(current tftypes.Value) tftypes.Value {
		if !isKnown {
			return tftypes.NewValue(current.Type(), tftypes.UnknownValue)
		}

		if predicted == nil {
			predicted = []tftypes.Value{}
		}

		return tftypes.NewValue(current.Type(), predicted)
	})
}

// publishedBindings returns the planned bindings as a live spec holds them once published, through the codec,
// reporting false while they hold unknown values or when creating the entry discovers bindings.
func (codec *Codec[D, S]) publishedBindings(planned tftypes.Value, created bool) ([]tftypes.Value, bool) {
	name, spec, err := codec.Spec(planned)

	if err != nil || created && addsNewBindings(spec) {
		return nil, false
	}

	value, err := codec.Value(name, spec)

	if err != nil {
		return nil, false
	}

	items, _, err := bindings(value)

	return items, err == nil
}

// sameBindings reports whether the planned bindings equal the stored ones.
func sameBindings(planned tftypes.Value, state tftypes.Value) bool {
	plannedBindings, plannedErr := attributeValue(planned, bindingsPath)

	stateBindings, stateErr := attributeValue(state, bindingsPath)

	return plannedErr == nil && stateErr == nil && plannedBindings.Equal(stateBindings)
}

// withPublishedLiveBindings sets live bindings left unknown by the plan from the published entry, null with a
// warning when it can't be read.
func (entryResource *entryResource[D, S]) withPublishedLiveBindings(ctx context.Context, name string, catalogName string, value tftypes.Value) (tftypes.Value, diag.Diagnostics) {
	var diagnostics diag.Diagnostics

	stored, err := attributeValue(value, liveBindingsPath)

	if err != nil || stored.IsKnown() {
		return value, diagnostics
	}

	live, err := entryResource.readLiveValue(ctx, name)

	if err != nil {
		diagnostics.AddWarning("Couldn't read the live bindings of "+entryResource.catalogType.name+" "+catalogName,
			err.Error()+"\n\n"+liveBindingsAttribute+" stays null until the next refresh.")

		value, err = setLiveBindings(value, func(current tftypes.Value) tftypes.Value {
			return tftypes.NewValue(current.Type(), nil)
		})
	} else {
		value, err = withLiveBindings(value, live)
	}

	if err != nil {
		diagnostics.AddError("Setting the live bindings of "+entryResource.catalogType.name+" "+catalogName, err.Error())
	}

	return value, diagnostics
}
