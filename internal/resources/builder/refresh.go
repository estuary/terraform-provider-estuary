package builder

import (
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strconv"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const bindingsAttribute string = "bindings"
const bindingTargetAttribute string = "target"
const bindingSourceAttribute string = "source"
const bindingNameAttribute string = "name"

// RefreshBindings returns state with each of its bindings refreshed from the live binding of the same target, and
// without the bindings no longer live, so plans show the changes made outside Terraform.
// Only the fields set in state are refreshed, the ones Estuary fills in staying unmanaged. Unmanaged appends the
// live bindings state doesn't hold, when Terraform manages every binding of the entry and the next apply removes them.
func (codec *Codec[D, S]) RefreshBindings(state tftypes.Value, live tftypes.Value, unmanaged bool) (tftypes.Value, error) {
	var refreshed []tftypes.Value = []tftypes.Value{}
	var managed map[string]bool = map[string]bool{}

	stateBindings, bindingsType, err := bindings(state)

	if err != nil || bindingsType == nil {
		return state, err
	}

	liveBindings, _, err := bindings(live)

	if err != nil {
		return state, err
	}

	var liveByTarget map[string]tftypes.Value = byTarget(liveBindings)
	var path *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(bindingsAttribute)

	for index, binding := range stateBindings {
		var target string = bindingTarget(binding)

		liveBinding, found := liveByTarget[target]

		if !found {
			continue
		}

		managed[target] = true

		refreshedBinding, err := codec.refreshManaged(path.WithElementKeyInt(index), binding, liveBinding)

		if err != nil {
			return state, err
		}

		refreshed = append(refreshed, refreshedBinding)
	}

	for _, binding := range liveBindings {
		if unmanaged && !managed[bindingTarget(binding)] {
			refreshed = append(refreshed, binding)
		}
	}

	return tftypes.Transform(state, func(attributePath *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		if !attributePath.Equal(path) {
			return current, nil
		}

		return tftypes.NewValue(bindingsType, refreshed), nil
	})
}

// refreshManaged returns the live value of the fields set in state, keeping the null ones, the secrets and their hash.
func (codec *Codec[D, S]) refreshManaged(path *tftypes.AttributePath, state tftypes.Value, live tftypes.Value) (tftypes.Value, error) {
	if state.IsNull() || !state.IsKnown() || codec.secretAttribute(path) {
		return state, nil
	}

	if live.IsNull() {
		if zeroValue(state) {
			return state, nil
		}

		return tftypes.NewValue(state.Type(), nil), nil
	}

	objectType, isObject := state.Type().(tftypes.Object)

	if !isObject {
		return live, nil
	}

	var stateAttributes, liveAttributes map[string]tftypes.Value
	var refreshed map[string]tftypes.Value = map[string]tftypes.Value{}

	if err := state.As(&stateAttributes); err != nil {
		return state, err
	}

	if err := live.As(&liveAttributes); err != nil {
		return state, err
	}

	for attribute := range objectType.AttributeTypes {
		value, err := codec.refreshManaged(path.WithAttributeName(attribute), stateAttributes[attribute], liveAttributes[attribute])

		if err != nil {
			return state, err
		}

		refreshed[attribute] = value
	}

	return tftypes.NewValue(objectType, refreshed), nil
}

// BindingChanges returns the changes publishing the planned binding at path makes to the live one it replaces, as
// "field: live -> planned" sorted by field, leaving the secrets and their hash out.
func (codec *Codec[D, S]) BindingChanges(path *tftypes.AttributePath, planned tftypes.Value, live tftypes.Value) ([]string, error) {
	var changes []string

	err := codec.bindingChanges(path, planned, live, &changes)

	return changes, err
}

func (codec *Codec[D, S]) bindingChanges(path *tftypes.AttributePath, planned tftypes.Value, live tftypes.Value, changes *[]string) error {
	if codec.secretAttribute(path) {
		return nil
	}

	if objectType, isObject := planned.Type().(tftypes.Object); isObject && planned.IsKnown() && !planned.IsNull() && !live.IsNull() {
		var plannedAttributes, liveAttributes map[string]tftypes.Value

		if err := planned.As(&plannedAttributes); err != nil {
			return err
		}

		if err := live.As(&liveAttributes); err != nil {
			return err
		}

		for _, attribute := range slices.Sorted(maps.Keys(objectType.AttributeTypes)) {
			if err := codec.bindingChanges(path.WithAttributeName(attribute), plannedAttributes[attribute], liveAttributes[attribute], changes); err != nil {
				return err
			}
		}

		return nil
	}

	if planned.Equal(live) || (planned.IsNull() && zeroValue(live)) || (live.IsNull() && zeroValue(planned)) {
		return nil
	}

	*changes = append(*changes, fmt.Sprintf("%s: %s -> %s", attributePath(path), displayValue(live), displayValue(planned)))

	return nil
}

// displayValue returns a scalar value as plans show it, (unset) when null.
func displayValue(value tftypes.Value) string {
	var text string
	var flag bool
	var number big.Float

	switch {
	case !value.IsKnown():
		return "(known after apply)"
	case value.IsNull():
		return "(unset)"
	case value.As(&text) == nil:
		return strconv.Quote(text)
	case value.As(&flag) == nil:
		return strconv.FormatBool(flag)
	case value.As(&number) == nil:
		return number.Text('f', -1)
	default:
		return "(changed)"
	}
}

// zeroValue reports whether value is the zero value of its type, which the spec omits.
func zeroValue(value tftypes.Value) bool {
	var text string
	var flag bool
	var number big.Float

	switch {
	case value.Type().Is(tftypes.String):
		return value.As(&text) == nil && text == ""
	case value.Type().Is(tftypes.Bool):
		return value.As(&flag) == nil && !flag
	case value.Type().Is(tftypes.Number):
		return value.As(&number) == nil && number.Sign() == 0
	default:
		return false
	}
}

// bindings returns the bindings of an entry value and their list type, a nil type when the entry or its bindings are null.
func bindings(value tftypes.Value) ([]tftypes.Value, tftypes.Type, error) {
	var attributes map[string]tftypes.Value
	var items []tftypes.Value

	if value.Type() == nil || value.IsNull() || !value.IsKnown() {
		return nil, nil, nil
	}

	if err := value.As(&attributes); err != nil {
		return nil, nil, err
	}

	list, found := attributes[bindingsAttribute]

	if !found || list.IsNull() || !list.IsKnown() {
		return nil, nil, nil
	}

	if err := list.As(&items); err != nil {
		return nil, nil, err
	}

	return items, list.Type(), nil
}

// byTarget returns the bindings by their target collection.
func byTarget(items []tftypes.Value) map[string]tftypes.Value {
	var indexed map[string]tftypes.Value = map[string]tftypes.Value{}

	for _, binding := range items {
		indexed[bindingTarget(binding)] = binding
	}

	return indexed
}

// bindingTarget returns the target collection of a capture binding value, empty when unknown.
func bindingTarget(binding tftypes.Value) string {
	var target string
	var value tftypes.Value = bindingAttribute(binding, bindingTargetAttribute)

	if value.IsKnown() && !value.IsNull() {
		_ = value.As(&target)
	}

	return target
}

// withBindingNames returns value with the name of each binding set to its collection, the target of a capture binding
// or the source of a materialization one, unknown while the collection is.
func withBindingNames(value tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(value, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		var steps []tftypes.AttributePathStep = path.Steps()
		var attributes map[string]tftypes.Value

		if len(steps) != 2 || steps[0] != tftypes.AttributeName(bindingsAttribute) || !current.IsKnown() || current.IsNull() {
			return current, nil
		}

		if err := current.As(&attributes); err != nil {
			return current, err
		}

		collection, isCapture := attributes[bindingTargetAttribute]

		if !isCapture {
			collection = bindingAttribute(current, bindingSourceAttribute)
		}

		attributes[bindingNameAttribute] = collection

		return tftypes.NewValue(current.Type(), attributes), nil
	})
}

// bindingAttribute returns an attribute of a binding value, unknown when the binding is.
func bindingAttribute(binding tftypes.Value, attribute string) tftypes.Value {
	var attributes map[string]tftypes.Value

	if !binding.IsKnown() {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}

	if binding.IsNull() || binding.As(&attributes) != nil {
		return tftypes.NewValue(tftypes.String, nil)
	}

	if value, found := attributes[attribute]; found {
		return value
	}

	return tftypes.NewValue(tftypes.String, nil)
}
