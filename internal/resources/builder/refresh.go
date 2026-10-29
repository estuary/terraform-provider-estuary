package builder

import (
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strconv"
	"sync"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Refresh returns state refreshed from live, so plans show changes made outside Terraform. Only attributes set in
// state refresh; bindings pair by collection. AppendUnheld appends live bindings state doesn't hold.
func (codec *Codec[D, S]) Refresh(state tftypes.Value, live tftypes.Value, appendUnheld bool) (tftypes.Value, error) {
	var refreshed []tftypes.Value = []tftypes.Value{}
	var held map[string]bool = map[string]bool{}
	var path *tftypes.AttributePath = bindingsPath

	state, err := codec.refreshManaged(tftypes.NewAttributePath(), state, live)

	if err != nil {
		return state, err
	}

	stateBindings, bindingsType, err := bindings(state)

	if err != nil || bindingsType == nil {
		return state, err
	}

	liveBindings, _, err := bindings(live)

	if err != nil {
		return state, err
	}

	var liveByCollection map[string]tftypes.Value = byCollection(liveBindings)

	for index, binding := range stateBindings {
		var collection string = bindingCollectionValue(binding)

		liveBinding, found := liveByCollection[collection]

		if !found {
			continue
		}

		held[collection] = true

		refreshedBinding, err := codec.refreshManaged(path.WithElementKeyInt(index), binding, liveBinding)

		if err != nil {
			return state, err
		}

		refreshed = append(refreshed, refreshedBinding)
	}

	for _, binding := range liveBindings {
		if appendUnheld && !held[bindingCollectionValue(binding)] {
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

// refreshManaged returns the live value of every field except secrets, provider-only attributes and bindings.
// Fields null in state stay null while live holds nothing, as Estuary fills in empty defaults.
func (codec *Codec[D, S]) refreshManaged(path *tftypes.AttributePath, state tftypes.Value, live tftypes.Value) (tftypes.Value, error) {
	if _, isBlock := bindingAttributeIndex(path, backfillAttribute); isBlock && live.IsNull() && !state.IsNull() && state.IsKnown() {
		var nulls map[string]tftypes.Value = map[string]tftypes.Value{}

		for attribute, attributeType := range state.Type().(tftypes.Object).AttributeTypes {
			nulls[attribute] = tftypes.NewValue(attributeType, nil)
		}

		live = tftypes.NewValue(state.Type(), nulls)
	}

	if path.Equal(tftypes.NewAttributePath().WithAttributeName(shardsAttribute)) && state.IsKnown() {
		return refreshedShards(state, live)
	}

	if !state.IsKnown() || codec.secrets.attribute(path) || providerOnly(path) || path.Equal(bindingsPath) {
		return state, nil
	}

	if state.IsNull() && holdsNothing(live) {
		return state, nil
	}

	if state.IsNull() {
		return live, nil
	}

	if live.IsNull() {
		if zeroValue(state) || codec.catalogType.omittedDefault != nil && codec.catalogType.omittedDefault(path, state) {
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

// refreshedShards returns every live shard setting, so plans show settings set outside Terraform. Zero values
// set in state stay when live omits them.
func refreshedShards(state tftypes.Value, live tftypes.Value) (tftypes.Value, error) {
	var stateAttributes map[string]tftypes.Value
	var refreshed map[string]tftypes.Value = map[string]tftypes.Value{}
	var attributeTypes map[string]tftypes.Type = map[string]tftypes.Type{}

	if !state.IsNull() && state.As(&stateAttributes) != nil {
		return live, nil
	}

	if !live.IsNull() {
		if err := live.As(&refreshed); err != nil {
			return state, err
		}
	}

	for key, value := range stateAttributes {
		if _, isLive := refreshed[key]; !isLive && zeroValue(value) {
			refreshed[key] = value
		}
	}

	if len(refreshed) == 0 {
		return tftypes.NewValue(state.Type(), nil), nil
	}

	for key, value := range refreshed {
		attributeTypes[key] = value.Type()
	}

	return tftypes.NewValue(tftypes.Object{AttributeTypes: attributeTypes}, refreshed), nil
}

// planShards returns the configured shards, else the live ones in state, null for an entry not created yet.
func planShards(plan tftypes.Value, config tftypes.Value, state tftypes.Value) (tftypes.Value, error) {
	var configured tftypes.Value = blockAttribute(config, shardsAttribute, tftypes.DynamicPseudoType)
	var stored tftypes.Value = tftypes.NewValue(tftypes.DynamicPseudoType, nil)

	if !configured.IsNull() || config.Type() == nil || config.IsNull() {
		return plan, nil
	}

	if state.Type() != nil && !state.IsNull() {
		stored = blockAttribute(state, shardsAttribute, tftypes.DynamicPseudoType)
	}

	return tftypes.Transform(plan, func(attributePath *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		if !attributePath.Equal(tftypes.NewAttributePath().WithAttributeName(shardsAttribute)) {
			return current, nil
		}

		return stored, nil
	})
}

// planBindingFields plans the fields of each binding configured without them as the live ones in state, else in the live spec for bindings Terraform takes over, null for new bindings.
func planBindingFields(plan tftypes.Value, config tftypes.Value, state tftypes.Value, readLive liveReader) (tftypes.Value, error) {
	configured, _, err := bindings(config)

	if err != nil {
		return plan, err
	}

	planned, _, err := bindings(plan)

	if err != nil {
		return plan, err
	}

	stored, err := indexedBindings(state)

	if err != nil {
		return plan, err
	}

	var readLiveBindings func() map[string]tftypes.Value = sync.OnceValue(func() map[string]tftypes.Value {
		live, found := readLive()

		if !found {
			return nil
		}

		indexed, _ := indexedBindings(live)

		return indexed
	})

	return tftypes.Transform(plan, func(attributePath *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		index, isFields := bindingAttributeIndex(attributePath, bindingFieldsAttribute)

		if !isFields || index >= len(configured) || index >= len(planned) ||
			!blockAttribute(configured[index], bindingFieldsAttribute, current.Type()).IsNull() {
			return current, nil
		}

		var collection string = bindingCollectionValue(planned[index])

		if kept, isStored := stored[collection]; isStored {
			return blockAttribute(kept, bindingFieldsAttribute, current.Type()), nil
		}

		if kept, isLive := readLiveBindings()[collection]; isLive && collection != "" {
			return blockAttribute(kept, bindingFieldsAttribute, current.Type()), nil
		}

		return tftypes.NewValue(current.Type(), nil), nil
	})
}

// BindingChanges returns the "field: live -> planned" changes publishing the binding at path makes, sorted,
// excluding secrets, hashes and provider-only attributes.
func (codec *Codec[D, S]) BindingChanges(path *tftypes.AttributePath, planned tftypes.Value, live tftypes.Value) ([]string, error) {
	var changes []string

	err := codec.bindingChanges(path, planned, live, &changes)

	return changes, err
}

func (codec *Codec[D, S]) bindingChanges(path *tftypes.AttributePath, planned tftypes.Value, live tftypes.Value, changes *[]string) error {
	if codec.secrets.attribute(path) || providerOnly(path) {
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

// displayValue returns a scalar as plans show it, (unset) when null.
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

// holdsNothing reports whether a live value is null, zero, empty, or an object holding nothing.
func holdsNothing(value tftypes.Value) bool {
	var attributes map[string]tftypes.Value
	var items []tftypes.Value

	if value.Type() == nil || value.IsNull() || !value.IsKnown() || zeroValue(value) {
		return true
	}

	if value.As(&attributes) == nil {
		for _, attribute := range attributes {
			if !holdsNothing(attribute) {
				return false
			}
		}

		return true
	}

	return value.As(&items) == nil && len(items) == 0
}

// zeroValue reports whether value is its type's zero value, which the spec omits.
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
