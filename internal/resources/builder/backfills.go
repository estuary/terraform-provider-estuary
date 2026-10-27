package builder

import (
	"fmt"
	"math/big"
	"reflect"
	"slices"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const dataflowResetStrategy string = "dataflowReset"

// captureBackfill is the capture backfill block: raising its trigger backfills every enabled binding. Not in the catalog spec.
type captureBackfill struct {
	Trigger  int    `yaml:"trigger,omitempty"`
	Strategy string `yaml:"strategy,omitempty" enum:"incremental,dataflowReset"`
}

// captureBindingBackfill is the capture binding backfill block: raising its trigger backfills the binding.
// Mode mirrors the resource's backfill mode, see backfillModeKey.
type captureBindingBackfill struct {
	Trigger  int    `yaml:"trigger,omitempty"`
	Strategy string `yaml:"strategy,omitempty" enum:"incremental,dataflowReset"`
	Mode     string `yaml:"mode,omitempty"`
}

// MaterializationBackfill is the materialization binding backfill block: raising its trigger backfills the binding.
type MaterializationBackfill struct {
	Trigger int `yaml:"trigger,omitempty"`
}

// backfillRequests returns the sorted collections to backfill and to reset, from raised binding triggers and, for
// every captureWide collection, a raised entry trigger. Each binding uses its own strategy, else the entry one.
func backfillRequests(plan tftypes.Value, prior tftypes.Value, captureWide []string) ([]string, []string, error) {
	var backfills []string
	var resets []string
	var requested map[string]bool = map[string]bool{}

	plannedBindings, _, err := bindings(plan)

	if err != nil {
		return nil, nil, err
	}

	previous, err := indexedBindings(prior)

	if err != nil {
		return nil, nil, err
	}

	var planned map[string]tftypes.Value = byCollection(plannedBindings)

	for collection, binding := range planned {
		priorBinding, managed := previous[collection]

		if collection != "" && (managed || slices.Contains(captureWide, collection)) && raisesTrigger(binding, priorBinding) {
			requested[collection] = true
		}
	}

	if prior.Type() != nil && !prior.IsNull() && raisesTrigger(plan, prior) {
		for _, collection := range captureWide {
			requested[collection] = true
		}
	}

	for collection := range requested {
		if bindingStrategy(plan, planned[collection]) == dataflowResetStrategy {
			resets = append(resets, collection)
		} else {
			backfills = append(backfills, collection)
		}
	}

	slices.Sort(backfills)
	slices.Sort(resets)

	return backfills, resets, nil
}

// captureWide returns the collections of the published bindings a raised entry trigger backfills: those bound live.
// Disabled bindings are left out, as the dashboard does.
func captureWide(published any, live []any) []string {
	var collections []string
	var bound []string

	for _, binding := range live {
		bound = append(bound, catalog.BindingCollection(binding))
	}

	items, _ := published.([]any)

	for _, binding := range items {
		object, _ := binding.(map[string]any)

		if disable, _ := object[disableAttribute].(bool); disable {
			continue
		}

		if collection := catalog.BindingCollection(binding); collection != "" && slices.Contains(bound, collection) {
			collections = append(collections, collection)
		}
	}

	return collections
}

// disabled reports whether a binding value sets disable.
func disabled(binding tftypes.Value) bool {
	disable, _ := known[bool](bindingAttribute(binding, disableAttribute))

	return disable
}

// bindingStrategy returns the backfill strategy of a binding value, else that of the entry.
func bindingStrategy(entry tftypes.Value, binding tftypes.Value) string {
	if strategy := backfillText(binding, backfillStrategyAttribute); strategy != "" {
		return strategy
	}

	return backfillText(entry, backfillStrategyAttribute)
}

// pendingResets returns the sorted enabled planned bindings whose backfill strategy, else the entry one,
// is dataflowReset: raising their trigger, or the entry one, resets their collections.
func pendingResets(plan tftypes.Value) ([]string, error) {
	var collections []string

	plannedBindings, _, err := bindings(plan)

	if err != nil {
		return nil, err
	}

	for _, binding := range plannedBindings {
		var collection string = bindingCollectionValue(binding)

		if collection != "" && !disabled(binding) && bindingStrategy(plan, binding) == dataflowResetStrategy {
			collections = append(collections, collection)
		}
	}

	slices.Sort(collections)

	return collections, nil
}

// backfillChanged reports whether the plan changes the entry backfill block or that of a binding.
func backfillChanged(plan tftypes.Value, prior tftypes.Value) bool {
	planned, _ := indexedBindings(plan)
	previous, _ := indexedBindings(prior)

	if backfillDiffers(plan, prior) {
		return true
	}

	for collection, binding := range planned {
		if backfillDiffers(binding, previous[collection]) {
			return true
		}
	}

	return false
}

// backfillDiffers reports whether the backfill blocks of two values differ, a missing block matching a null one.
func backfillDiffers(planned tftypes.Value, prior tftypes.Value) bool {
	var plannedBackfill tftypes.Value = bindingAttribute(planned, backfillAttribute)
	var priorBackfill tftypes.Value = bindingAttribute(prior, backfillAttribute)

	if plannedBackfill.IsNull() && priorBackfill.IsNull() {
		return false
	}

	return !plannedBackfill.Equal(priorBackfill)
}

// raisesTrigger reports whether the planned backfill trigger is higher than the prior one, or newly set.
func raisesTrigger(planned tftypes.Value, prior tftypes.Value) bool {
	return raisesValue(backfillSetting(planned, backfillTriggerAttribute), prior)
}

// triggersUnmanaged reports whether the plan sets a backfill trigger on a binding missing from the prior state.
func triggersUnmanaged(plan tftypes.Value, prior tftypes.Value) bool {
	planned, _ := indexedBindings(plan)
	previous, _ := indexedBindings(prior)

	for collection, binding := range planned {
		if _, managed := previous[collection]; !managed && raisesTrigger(binding, tftypes.Value{}) {
			return true
		}
	}

	return false
}

// raisesValue reports whether trigger is higher than the prior backfill trigger, or newly set.
func raisesValue(trigger tftypes.Value, prior tftypes.Value) bool {
	planned, set := integer(trigger)
	previous, wasSet := integer(backfillSetting(prior, backfillTriggerAttribute))

	return set && (!wasSet || planned > previous)
}

// validateBackfillTriggers validates the entry and binding backfill triggers, see validateBackfillTrigger.
func validateBackfillTriggers(plan tftypes.Value, state tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	plannedBindings, _, err := bindings(plan)

	if err != nil {
		return diagnostics
	}

	previous, err := indexedBindings(state)

	if err != nil {
		return diagnostics
	}

	diagnostics.Append(validateBackfillTrigger(path.Root(backfillAttribute).AtName(backfillTriggerAttribute), "the capture", plan, state)...)

	for index, binding := range plannedBindings {
		var collection string = bindingCollectionValue(binding)
		var attributePath path.Path = path.Root(bindingsAttribute).AtListIndex(index).AtName(backfillAttribute).AtName(backfillTriggerAttribute)

		diagnostics.Append(validateBackfillTrigger(attributePath, collection, binding, previous[collection])...)

		if disabled(binding) && raisesTrigger(binding, previous[collection]) {
			diagnostics.AddAttributeError(attributePath, "Backfill trigger raised on a disabled binding",
				fmt.Sprintf("backfill.trigger of %s is raised, but the binding is disabled, so there is nothing to backfill. "+
					"Set disable = false or remove the disable property in the same change to enable and backfill it, or keep the trigger at its current value.", collection))
		}
	}

	return diagnostics
}

// validateBackfillTrigger rejects a backfill trigger that is unknown at plan time, under 1, or lower than the prior one.
func validateBackfillTrigger(attributePath path.Path, owner string, planned tftypes.Value, prior tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	if planned.Type() == nil || !planned.IsKnown() || planned.IsNull() {
		return diagnostics
	}

	if !backfillSetting(planned, backfillTriggerAttribute).IsKnown() {
		diagnostics.AddAttributeError(attributePath, "Backfill trigger unknown at plan time",
			"backfill.trigger must be known at plan time, as a number written in the configuration or a variable. "+
				"A value only known at apply time, such as timestamp(), would backfill on every apply.")

		return diagnostics
	}

	trigger, set := integer(backfillSetting(planned, backfillTriggerAttribute))

	if !set {
		return diagnostics
	}

	if trigger < 1 {
		diagnostics.AddAttributeError(attributePath, "Invalid backfill trigger",
			fmt.Sprintf("backfill.trigger is %d, it must be 1 or greater.", trigger))

		return diagnostics
	}

	current, wasSet := integer(backfillSetting(prior, backfillTriggerAttribute))

	if wasSet && trigger < current {
		diagnostics.AddAttributeError(attributePath, "Backfill trigger lowered",
			fmt.Sprintf("backfill.trigger of %s is %d, lower than its current %d. It only goes up: keep %d, or raise it "+
				"to backfill once more.", owner, trigger, current, current))
	}

	return diagnostics
}

// modeBackfills returns the planned bindings whose backfill mode, under key, differs from the live one, mapped to
// "live -> planned". The connector requires a backfill with each change.
func modeBackfills(planned any, live []any, key string) map[string]string {
	var changes map[string]string = map[string]string{}
	var modes map[string]string = map[string]string{}

	if key == "" {
		return changes
	}

	for _, binding := range live {
		mode, _ := utils.Lookup(binding, "resource")[key].(string)

		modes[catalog.BindingCollection(binding)] = mode
	}

	bindings, _ := planned.([]any)

	for _, binding := range bindings {
		var collection string = catalog.BindingCollection(binding)

		mode, _ := utils.Lookup(binding, "resource")[key].(string)
		liveMode, isLive := modes[collection]

		if isLive && mode != liveMode {
			changes[collection] = fmt.Sprintf("%q -> %q", liveMode, mode)
		}
	}

	return changes
}

// unbackfilledModes returns the mode changes, see modeBackfills, whose binding the plan doesn't backfill or reset.
func unbackfilledModes(planned any, live []any, key string, plan tftypes.Value, prior tftypes.Value, captureWide []string) (map[string]string, error) {
	var missing map[string]string = map[string]string{}

	backfills, resets, err := backfillRequests(plan, prior, captureWide)

	if err != nil {
		return nil, err
	}

	for collection, change := range modeBackfills(planned, live, key) {
		if !slices.Contains(backfills, collection) && !slices.Contains(resets, collection) {
			missing[collection] = change
		}
	}

	return missing, nil
}

// keepBackfills copies the live backfill counter onto each planned binding, plus one for backfills, so publishing
// never lowers it.
func keepBackfills(planned any, live []any, backfills []string) {
	var counters map[string]int = map[string]int{}

	for _, binding := range live {
		object, _ := binding.(map[string]any)
		counter, _ := object[backfillAttribute].(int)

		counters[catalog.BindingCollection(binding)] = counter
	}

	bindings, _ := planned.([]any)

	for _, binding := range bindings {
		object, _ := binding.(map[string]any)

		if object == nil {
			continue
		}

		var collection string = catalog.BindingCollection(binding)
		var counter int = counters[collection]

		if slices.Contains(backfills, collection) {
			counter++
		}

		if counter == 0 {
			delete(object, backfillAttribute)
		} else {
			object[backfillAttribute] = counter
		}
	}
}

// backfillSetting returns an attribute of the owner's backfill block.
func backfillSetting(owner tftypes.Value, attribute string) tftypes.Value {
	if owner.Type() == nil {
		return tftypes.NewValue(tftypes.DynamicPseudoType, nil)
	}

	return blockAttribute(bindingAttribute(owner, backfillAttribute), attribute, tftypes.DynamicPseudoType)
}

// integer converts a number value, reporting false when null or unknown.
func integer(value tftypes.Value) (int64, bool) {
	number, isNumber := known[big.Float](value)

	if !isNumber {
		return 0, false
	}

	converted, _ := number.Int64()

	return converted, true
}

// backfillText returns a string attribute of the owner's backfill block, empty when unset.
func backfillText(owner tftypes.Value, attribute string) string {
	text, _ := known[string](backfillSetting(owner, attribute))

	return text
}

// backfillModeKey returns the YAML key of the R field tagged backfill:"mode", or empty when there is none.
func backfillModeKey[R any]() string {
	var resourceType reflect.Type = reflect.TypeFor[R]()

	if resourceType.Kind() != reflect.Struct {
		return ""
	}

	for index := range resourceType.NumField() {
		if field := resourceType.Field(index); field.Tag.Get(backfillAttribute) == backfillModeAttribute {
			return catalog.FieldKey(field)
		}
	}

	return ""
}

// moveField moves from.fromKey to to.toKey in each binding, creating the to object if missing.
func moveField(bindings any, from string, fromKey string, to string, toKey string) {
	items, _ := bindings.([]any)

	for _, item := range items {
		binding, _ := item.(map[string]any)
		value, found := utils.Lookup(binding, from)[fromKey]

		if binding == nil || !found {
			continue
		}

		if utils.Lookup(binding, to) == nil {
			binding[to] = map[string]any{}
		}

		utils.Lookup(binding, to)[toKey] = value

		delete(utils.Lookup(binding, from), fromKey)
	}
}
