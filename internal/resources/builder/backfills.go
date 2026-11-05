package builder

import (
	"errors"
	"fmt"
	"maps"
	"math/big"
	"reflect"
	"slices"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder/translator"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// backfillStrategy is how a raised capture backfill trigger backfills a binding. Not in the catalog spec.
type backfillStrategy string

// captureBackfill is the capture backfill block: raising its trigger backfills every enabled binding. Not in the catalog spec.
type captureBackfill struct {
	Trigger  int              `yaml:"trigger,omitempty" description:"Raise it to backfill every enabled binding of the capture, including those Terraform doesn't manage. A provider control Estuary never sees, not its backfill counter: each raise adds one to each binding's live counter, or resets its collection with 'dataflowReset', leaving its counter as is. Starts at 1 and only goes up."`
	Strategy backfillStrategy `yaml:"strategy,omitempty" enum:"incremental,dataflowReset" description:"Default backfill strategy of the bindings: 'incremental' (default) captures again into the existing collection, 'dataflowReset' resets the collection, backfilling its materializations and truncating their tables. Changing it alone publishes nothing."`
}

// captureBindingBackfill is the capture binding backfill block: raising its trigger backfills the binding.
// Mode mirrors the resource's backfill mode, the field tagged backfill:"mode".
type captureBindingBackfill struct {
	Trigger  int              `yaml:"trigger,omitempty" description:"Raise it to backfill the binding: capture the whole resource again. A provider control Estuary never sees, not its backfill counter: each raise adds one to the live counter, or resets the collection with 'dataflowReset', leaving the counter as is. Starts at 1 and only goes up, and can't be raised on a disabled binding."`
	Strategy backfillStrategy `yaml:"strategy,omitempty" enum:"incremental,dataflowReset" description:"Overrides the backfill.strategy of the capture for this binding: 'incremental' or 'dataflowReset'. Changing it alone publishes nothing."`
	Mode     string           `yaml:"mode,omitempty"`
}

const (
	incrementalStrategy   backfillStrategy = "incremental"
	dataflowResetStrategy backfillStrategy = "dataflowReset"
)

// backfillStrategies lists every backfill strategy, useful for schema validators.
var backfillStrategies []backfillStrategy = []backfillStrategy{
	incrementalStrategy,
	dataflowResetStrategy,
}

// MaterializationBackfill is the materialization and binding backfill block: raising its trigger backfills the binding,
// or every enabled binding of the materialization. Not in the catalog spec.
type MaterializationBackfill struct {
	Trigger int `yaml:"trigger,omitempty" description:"Raise it to backfill the binding, or every enabled binding of the materialization: each binding is materialized again from its collection. A provider control Estuary never sees, not its backfill counter: each raise adds one to the live counter. Starts at 1 and only goes up."`
}

// backfillRequests returns the published bindings to backfill, by index, and the sorted collections to reset: those
// of the planned bindings raising their trigger and, when the entry trigger is raised, of every wide binding. Each
// binding uses its own strategy, else the entry one. A reset backfills every binding of its collection, so those
// aren't backfilled again.
func (pairing bindingPairing) backfillRequests(published []publishedBinding, wide []int) ([]int, []string) {
	var backfills []int
	var resets []string
	var requested []int

	plannedBindings, priorBindings := pairing.priorCounterparts()

	for index, binding := range published {
		if raisesTrigger(binding.plannedValue(plannedBindings), binding.plannedValue(priorBindings)) {
			requested = append(requested, index)
		}
	}

	if raisesTrigger(pairing.plan, pairing.state) {
		requested = append(requested, wide...)
	}

	slices.Sort(requested)

	for _, index := range slices.Compact(requested) {
		if bindingStrategy(pairing.plan, published[index].plannedValue(plannedBindings)) == dataflowResetStrategy {
			resets = append(resets, catalog.BindingCollection(published[index].document))
		} else {
			backfills = append(backfills, index)
		}
	}

	slices.Sort(resets)

	resets = slices.Compact(resets)

	backfills = slices.DeleteFunc(backfills, func(index int) bool {
		return slices.Contains(resets, catalog.BindingCollection(published[index].document))
	})

	return backfills, resets
}

// entryWide returns the indexes of the published bindings a raised entry trigger backfills: those bound live.
// Disabled bindings are left out, as the dashboard does.
func entryWide(published []publishedBinding) []int {
	var wide []int

	for index, binding := range published {
		if binding.live != -1 && !catalog.BindingDisabled(binding.document) {
			wide = append(wide, index)
		}
	}

	return wide
}

// bindingCollections returns the sorted distinct collections of the published bindings at indexes.
func bindingCollections(published []publishedBinding, indexes []int) []string {
	var collections []string

	for _, index := range indexes {
		collections = append(collections, catalog.BindingCollection(published[index].document))
	}

	slices.Sort(collections)

	return slices.Compact(collections)
}

// disabled reports whether a binding value sets disable.
func disabled(binding tftypes.Value) bool {
	disable, _ := known[bool](blockAttribute(binding, disableAttribute, tftypes.Bool))

	return disable
}

// bindingStrategy returns the backfill strategy of a binding value, else that of the entry.
func bindingStrategy(entry tftypes.Value, binding tftypes.Value) backfillStrategy {
	if strategy := backfillText(binding, backfillStrategyAttribute); strategy != "" {
		return backfillStrategy(strategy)
	}

	return backfillStrategy(backfillText(entry, backfillStrategyAttribute))
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

	return slices.Compact(collections), nil
}

// backfillChanged reports whether the plan changes the entry backfill block or that of a binding.
func (pairing bindingPairing) backfillChanged() bool {
	if backfillDiffers(pairing.plan, pairing.state) {
		return true
	}

	plannedBindings, priorBindings := pairing.priorCounterparts()

	for index, binding := range plannedBindings {
		if backfillDiffers(binding, priorBindings[index]) {
			return true
		}
	}

	return false
}

// raisesTriggers reports whether the plan raises the entry backfill trigger or that of a binding.
func (pairing bindingPairing) raisesTriggers() bool {
	if raisesTrigger(pairing.plan, pairing.state) {
		return true
	}

	plannedBindings, priorBindings := pairing.priorCounterparts()

	for index, binding := range plannedBindings {
		if raisesTrigger(binding, priorBindings[index]) {
			return true
		}
	}

	return false
}

// backfillDiffers reports whether the backfill blocks of two values differ, a missing block matching a null one.
func backfillDiffers(planned tftypes.Value, prior tftypes.Value) bool {
	var plannedBackfill tftypes.Value = blockAttribute(planned, backfillAttribute, tftypes.DynamicPseudoType)
	var priorBackfill tftypes.Value = blockAttribute(prior, backfillAttribute, tftypes.DynamicPseudoType)

	if plannedBackfill.IsNull() && priorBackfill.IsNull() {
		return false
	}

	return !plannedBackfill.Equal(priorBackfill)
}

// raisesTrigger reports whether the planned backfill trigger is higher than the prior one. A trigger the prior doesn't
// set, as after a create, an import, a takeover or a new backfill block, is its starting value and backfills nothing.
func raisesTrigger(planned tftypes.Value, prior tftypes.Value) bool {
	trigger, set := integer(backfillSetting(planned, backfillTriggerAttribute))
	previous, wasSet := integer(backfillSetting(prior, backfillTriggerAttribute))

	return set && wasSet && trigger > previous
}

// validateBackfillTriggers validates the entry and binding backfill triggers, see validateBackfillTrigger.
func validateBackfillTriggers(pairing bindingPairing, catalogType catalogType) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	plannedBindings, priorBindings := pairing.priorCounterparts()

	diagnostics.Append(validateBackfillTrigger(path.Root(backfillAttribute).AtName(backfillTriggerAttribute), "the "+catalogType.name, pairing.plan, pairing.state)...)

	for index, binding := range plannedBindings {
		var collection string = bindingCollectionValue(binding)
		var attributePath path.Path = path.Root(bindingsAttribute).AtListIndex(index).AtName(backfillAttribute).AtName(backfillTriggerAttribute)

		diagnostics.Append(validateBackfillTrigger(attributePath, collection, binding, priorBindings[index])...)

		if disabled(binding) && raisesTrigger(binding, priorBindings[index]) {
			diagnostics.AddAttributeError(attributePath, "Backfill trigger raised on a disabled binding",
				fmt.Sprintf("backfill.trigger of %s is raised, but the binding is disabled, so there is nothing to backfill. "+
					"To enable and backfill it, set disable = false or remove disable in the same change. Otherwise, keep the trigger at its current value.", collection))
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
			"backfill.trigger must be known at plan time: write a number in the configuration or use a variable. "+
				"A value only known at apply time, such as timestamp(), would backfill on every apply.")

		return diagnostics
	}

	trigger, set := integer(backfillSetting(planned, backfillTriggerAttribute))

	if !set {
		return diagnostics
	}

	if trigger < 1 {
		diagnostics.AddAttributeError(attributePath, "Invalid backfill trigger",
			fmt.Sprintf("backfill.trigger is %d, but it must be 1 or greater.", trigger))

		return diagnostics
	}

	current, wasSet := integer(backfillSetting(prior, backfillTriggerAttribute))

	if wasSet && trigger < current {
		diagnostics.AddAttributeError(attributePath, "Backfill trigger lowered",
			fmt.Sprintf("backfill.trigger of %s is %d, which is lower than its current value, %d. The trigger can only go up: keep it "+
				"at %d, or raise it to backfill again.", owner, trigger, current, current))
	}

	return diagnostics
}

// modeBackfills returns the planned published bindings, by index, whose backfill mode, under key, differs from the
// live one, mapped to "live -> planned". The connector requires a backfill with each change.
func modeBackfills(published []publishedBinding, live []any, key string) map[int]string {
	var changes map[int]string = map[int]string{}

	if key == "" {
		return changes
	}

	for index, binding := range published {
		var liveBinding map[string]any = binding.liveBinding(live)

		mode, _ := catalog.BindingResource(binding.document)[key].(string)
		liveMode, _ := catalog.BindingResource(liveBinding)[key].(string)

		if binding.planned != -1 && liveBinding != nil && mode != liveMode {
			changes[index] = fmt.Sprintf("%q -> %q", liveMode, mode)
		}
	}

	return changes
}

// missingBackfill is a binding change the connector only accepts along with a backfill, which the plan doesn't raise.
type missingBackfill struct {
	binding   int      // index of the planned binding
	attribute []string // attribute names from the binding to the changed one
	summary   string
	detail    string
}

// missingModeBackfills returns the mode changes, see modeBackfills, whose binding is neither backfilled nor reset.
func missingModeBackfills(published []publishedBinding, changes map[int]string, backfills []int, resets []string) []missingBackfill {
	var missing []missingBackfill

	for _, index := range slices.Sorted(maps.Keys(changes)) {
		var collection string = catalog.BindingCollection(published[index].document)

		if slices.Contains(backfills, index) || slices.Contains(resets, collection) {
			continue
		}

		missing = append(missing, missingBackfill{
			binding:   published[index].planned,
			attribute: []string{backfillAttribute, backfillModeAttribute},
			summary:   "Backfill mode changed without a backfill",
			detail: "backfill.mode of " + collection + " changes " + changes[index] + ". The connector requires the binding " +
				"to be backfilled when its mode changes: raise its backfill.trigger, or the capture's, in the same change.",
		})
	}

	return missing
}

// missingStandardUpdatesBackfills returns the enabled planned bindings switching from live delta updates, under key, to
// standard updates without a backfill, which the connector rejects as it can't merge the rows a key already has.
func missingStandardUpdatesBackfills(published []publishedBinding, live []any, key string, backfills []int) []missingBackfill {
	var missing []missingBackfill

	if key == "" {
		return missing
	}

	var attribute string = translator.AttributeName(key)

	for index, binding := range published {
		var liveBinding map[string]any = binding.liveBinding(live)
		var collection string = catalog.BindingCollection(binding.document)

		delta, _ := catalog.BindingResource(binding.document)[key].(bool)
		liveDelta, _ := catalog.BindingResource(liveBinding)[key].(bool)

		if binding.planned == -1 || !liveDelta || delta || catalog.BindingDisabled(binding.document) || catalog.BindingDisabled(liveBinding) || slices.Contains(backfills, index) {
			continue
		}

		missing = append(missing, missingBackfill{
			binding:   binding.planned,
			attribute: []string{bindingResourceAttribute, attribute},
			summary:   "Delta updates disabled without a backfill",
			detail: "resource." + attribute + " of " + collection + " is true in Estuary and false in the plan. The connector " +
				"requires the binding to be backfilled when it switches from delta to standard updates, as the table holds several " +
				"rows per key that standard updates can't merge: raise its backfill.trigger, or the materialization's, in the same change.",
		})
	}

	return missing
}

// missingBackfillsError returns the error of the binding changes no backfill covers, nil without any.
func missingBackfillsError(missing []missingBackfill) error {
	var errs []error

	for _, change := range missing {
		errs = append(errs, errors.New(change.detail))
	}

	return errors.Join(errs...)
}

// keepBackfills copies the live backfill counter onto each published binding, plus one for backfills, by index, so
// publishing never lowers it.
func keepBackfills(published []publishedBinding, live []any, backfills []int) {
	for index, binding := range published {
		object, _ := binding.document.(map[string]any)

		if object == nil {
			continue
		}

		counter, _ := binding.liveBinding(live)[catalog.BackfillKey].(int)

		if slices.Contains(backfills, index) {
			counter++
		}

		if counter == 0 {
			delete(object, catalog.BackfillKey)
		} else {
			object[catalog.BackfillKey] = counter
		}
	}
}

// backfillSetting returns an attribute of the owner's backfill block.
func backfillSetting(owner tftypes.Value, attribute string) tftypes.Value {
	if owner.Type() == nil {
		return tftypes.NewValue(tftypes.DynamicPseudoType, nil)
	}

	return blockAttribute(blockAttribute(owner, backfillAttribute, tftypes.DynamicPseudoType), attribute, tftypes.DynamicPseudoType)
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

// taggedKey returns the YAML key of the first field of struct T tagged, or empty when there is none.
func taggedKey[T any](tagged func(field reflect.StructField) bool) string {
	var structType reflect.Type = reflect.TypeFor[T]()

	if structType.Kind() != reflect.Struct {
		return ""
	}

	for index := range structType.NumField() {
		if field := structType.Field(index); tagged(field) {
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
