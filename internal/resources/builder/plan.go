package builder

import (
	"fmt"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// warnDisabledLive warns when state is disabled live but config doesn't set shards.disable, which plans don't show.
func (entryResource *entryResource[D, S]) warnDisabledLive(config tftypes.Value, state tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	configured := bindingAttribute(bindingAttribute(config, shardsAttribute), disableAttribute)
	disabled, _ := known[bool](bindingAttribute(bindingAttribute(state, shardsAttribute), disableAttribute))

	if !configured.IsKnown() || !configured.IsNull() || !disabled {
		return diagnostics
	}

	diagnostics.AddAttributeWarning(path.Root(shardsAttribute), entryResource.warningSummary("Disabled outside Terraform", state),
		"shards.disable is true in Estuary, set outside Terraform, such as in the dashboard, and the configuration doesn't "+
			"set it, so the apply keeps the "+entryResource.entity+" disabled. Set shards = { disable = false } to enable it, or "+
			"shards = { disable = true } to manage it as disabled.")

	return diagnostics
}

// validateModeBackfills rejects backfill mode changes whose binding isn't backfilled, as the connector requires.
// Failing to read the live spec skips the check.
func (entryResource *entryResource[D, S]) validateModeBackfills(plan tftypes.Value, state tftypes.Value, readLive liveReader) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	holder, isHolder := any(*new(D)).(backfillModeHolder)

	if !isHolder || holder.resourceBackfillMode() == "" {
		return diagnostics
	}

	planned, live, wide, found := entryResource.planWide(plan, state, readLive)

	if !found {
		return diagnostics
	}

	missing, err := unbackfilledModes(planned, live, holder.resourceBackfillMode(), plan, state, wide)

	if err != nil {
		return diagnostics
	}

	plannedBindings, _, _ := bindings(plan)

	for index, binding := range plannedBindings {
		if change, isMissing := missing[bindingCollectionValue(binding)]; isMissing {
			diagnostics.AddAttributeError(path.Root(bindingsAttribute).AtListIndex(index).AtName(backfillAttribute).AtName(backfillModeAttribute),
				"Backfill mode changed without a backfill", modeBackfillDetail(bindingCollectionValue(binding), change))
		}
	}

	return diagnostics
}

// planWide returns the planned and live binding documents and the captureWide collections of the plan, with the
// unmanaged live bindings when bindings are generated. It reports false when the live spec or plan can't be read.
func (entryResource *entryResource[D, S]) planWide(plan tftypes.Value, state tftypes.Value, readLive liveReader) ([]any, []any, []string, bool) {
	live, found := readLive()

	if !found {
		return nil, nil, nil, false
	}

	_, spec, err := entryResource.codec.Spec(plan)

	if err != nil {
		return nil, nil, nil, false
	}

	plannedDocument, err := utils.YAMLDocument(spec)

	if err != nil {
		return nil, nil, nil, false
	}

	priorDocument, err := entryResource.codec.Document(state)

	if err != nil {
		return nil, nil, nil, false
	}

	liveDocument, err := entryResource.codec.Document(live)

	if err != nil {
		return nil, nil, nil, false
	}

	planned, _ := plannedDocument[bindingsAttribute].([]any)
	previous, _ := priorDocument[bindingsAttribute].([]any)
	liveBindings, _ := liveDocument[bindingsAttribute].([]any)

	var published []any = planned

	if generatesBindings(spec) {
		published = mergeBindings(planned, previous, liveBindings)
	}

	return planned, liveBindings, captureWide(published, liveBindings), true
}

// modeBackfillDetail explains the backfill a changed backfill mode requires.
func modeBackfillDetail(collection string, change string) string {
	return "backfill.mode of " + collection + " changes " + change + ", and the connector requires a backfill of the binding " +
		"along with it. Raise its backfill.trigger, or the capture's, in the same change."
}

// warnDataflowResets warns once about every collection the plan dataflow resets, which truncates its destination
// tables. Failing to read the live bindings skips the warning.
func (entryResource *entryResource[D, S]) warnDataflowResets(plan tftypes.Value, state tftypes.Value, readLive liveReader) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var wide []string
	var found bool

	if raisesTrigger(plan, state) || triggersUnmanaged(plan, state) {
		if _, _, wide, found = entryResource.planWide(plan, state, readLive); !found {
			return diagnostics
		}
	}

	_, resets, err := backfillRequests(plan, state, wide)

	if err != nil {
		return diagnostics
	}

	if len(resets) == 0 {
		return entryResource.warnPendingResets(plan, state)
	}

	diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Dataflow reset", plan),
		"A raised backfill.trigger resolves to backfill.strategy = \"dataflowReset\" for these bindings, so the apply resets "+
			"their collections:\n\n- "+strings.Join(resets, "\n- ")+"\n\n"+
			"Their data is replaced by a fresh backfill from the source and their inferred schemas are cleared. Every materialization "+
			"of them backfills, truncating its destination tables, and a materialization with on_incompatible_schema_change = \"abort\" "+
			"fails the apply. Raising the backfill.trigger of the "+entryResource.entity+" covers every enabled binding, those "+
			"Terraform doesn't manage included.")

	return diagnostics
}

// warnPendingResets warns, when the plan changes a backfill block without resetting, about the bindings whose strategy
// resolves to dataflowReset, so a later raise of a trigger doesn't reset them unnoticed.
func (entryResource *entryResource[D, S]) warnPendingResets(plan tftypes.Value, state tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	if !backfillChanged(plan, state) {
		return diagnostics
	}

	pending, err := pendingResets(plan)

	if err != nil || len(pending) == 0 {
		return diagnostics
	}

	diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Dataflow reset strategy", plan),
		"backfill.strategy resolves to \"dataflowReset\" for these bindings:\n\n- "+strings.Join(pending, "\n- ")+"\n\n"+
			"This apply doesn't reset them, but the next raise of their backfill.trigger, or the "+entryResource.entity+"'s, "+
			"does: their collections are replaced by a fresh backfill from the source and every materialization of them "+
			"truncates its destination tables. Set backfill.strategy = \"incremental\" on a binding to backfill it without a reset.")

	return diagnostics
}

// warnDiscoveredBindings warns when creating the entry adds discovered resources Terraform won't manage.
// It skips plans holding values it can't decode yet.
func (entryResource *entryResource[D, S]) warnDiscoveredBindings(plan tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	value, err := withoutTargets(plan)

	if err != nil {
		return diagnostics
	}

	_, spec, err := entryResource.codec.Spec(value)

	if err != nil || !addsNewBindings(spec) {
		return diagnostics
	}

	diagnostics.AddAttributeWarning(path.Root("auto_discover").AtName("add_new_bindings"), entryResource.warningSummary("Auto discovery adds unmanaged bindings", plan),
		"Creating the "+entryResource.entity+" adds a binding for every discovered resource not declared in bindings, as the dashboard "+
			"does, and later discoveries keep adding new ones. Terraform doesn't manage them: declare a binding to manage it.")

	return diagnostics
}

// warnRemovedBindings warns about removed bindings that discovery may add back. It skips unknown targets.
func (entryResource *entryResource[D, S]) warnRemovedBindings(plan tftypes.Value, state tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var planned map[string]bool = map[string]bool{}
	var removed []string

	_, spec, err := entryResource.codec.Spec(plan)

	if err != nil || !addsNewBindings(spec) {
		return diagnostics
	}

	plannedBindings, _, err := bindings(plan)

	if err != nil {
		return diagnostics
	}

	prior, _, err := bindings(state)

	if err != nil {
		return diagnostics
	}

	for _, binding := range plannedBindings {
		if !bindingAttribute(binding, bindingTargetAttribute).IsKnown() {
			return diagnostics
		}

		planned[bindingCollectionValue(binding)] = true
	}

	for _, binding := range prior {
		var target string = bindingCollectionValue(binding)

		if target != "" && !planned[target] {
			removed = append(removed, "- "+target)
		}
	}

	if len(removed) > 0 {
		diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Removed bindings may be added back", plan),
			"These bindings are removed from the "+entryResource.entity+", but auto discovery adds new bindings, so the next discovery "+
				"may add them back while their resources exist:\n\n"+strings.Join(removed, "\n")+
				"\n\nTo stop capturing one, keep its binding with disable = true instead.")
	}

	return diagnostics
}

// warnTakenOverBindings warns about unmanaged live bindings the plan shows as added, which the apply updates in
// place, listing the changed fields. Failing to read the live spec skips the warning.
func (entryResource *entryResource[D, S]) warnTakenOverBindings(plan tftypes.Value, state tftypes.Value, readLive liveReader) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var managed map[string]bool = map[string]bool{}
	var candidates map[int]string = map[int]string{}
	var takenOver []string

	planned, _, err := bindings(plan)

	if err != nil {
		return diagnostics
	}

	prior, _, err := bindings(state)

	if err != nil {
		return diagnostics
	}

	for _, binding := range prior {
		managed[bindingCollectionValue(binding)] = true
	}

	for index, binding := range planned {
		if target := bindingCollectionValue(binding); target != "" && !managed[target] {
			candidates[index] = target
		}
	}

	if len(candidates) == 0 {
		return diagnostics
	}

	live, found := readLive()

	if !found {
		return diagnostics
	}

	liveBindings, _, err := bindings(live)

	if err != nil {
		return diagnostics
	}

	var liveByCollection map[string]tftypes.Value = byCollection(liveBindings)

	for index, binding := range planned {
		target, isCandidate := candidates[index]
		liveBinding, isLive := liveByCollection[target]

		if !isCandidate || !isLive {
			continue
		}

		changes, err := entryResource.codec.BindingChanges(tftypes.NewAttributePath().WithAttributeName(bindingsAttribute).WithElementKeyInt(index), binding, liveBinding)

		if err != nil {
			continue
		}

		var entry string = fmt.Sprintf("- %s (bindings[%d]): no field changes", target, index)

		if len(changes) > 0 {
			entry = fmt.Sprintf("- %s (bindings[%d]):\n    %s", target, index, strings.Join(changes, "\n    "))
		}

		takenOver = append(takenOver, entry)
	}

	if len(takenOver) > 0 {
		diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Existing bindings taken over", plan),
			"These bindings are already bound in the "+entryResource.entity+". Terraform starts managing them and updates them in place, "+
				"rather than adding them, only changing the fields listed against the live binding:\n\n"+strings.Join(takenOver, "\n")+
				"\n\nThis is expected the first time Terraform manages an existing binding, and no action is needed.")
	}

	return diagnostics
}

// planSecretsHash plans the hash beside each write-only secret and warns about changed ones, as plans never show
// write-only values. It returns plan as is on error.
func (entryResource *entryResource[D, S]) planSecretsHash(plan tftypes.Value, config tftypes.Value, state tftypes.Value) (tftypes.Value, diag.Diagnostics) {
	var diagnostics diag.Diagnostics

	planned, changed, err := entryResource.codec.secrets.planHashes(plan, config, state)

	if err != nil {
		diagnostics.AddError("Planning "+entryResource.entity+" secrets", err.Error())
		return plan, diagnostics
	}

	for _, secret := range changed {
		diagnostics.AddAttributeWarning(attributePath(secret), entryResource.warningSummary("Secret changed: "+attributePath(secret).String(), plan),
			attributePath(secret).String()+" changed. The "+entryResource.entity+" is published with the new value.")
	}

	return planned, diagnostics
}

// warningSummary appends the catalog name to summary, as Terraform merges same-summary warnings across resources.
func (entryResource *entryResource[D, S]) warningSummary(summary string, plan tftypes.Value) string {
	name, isKnown := known[string](bindingAttribute(plan, nameAttribute))

	if !isKnown {
		return summary
	}

	return summary + " in " + entryResource.catalogName(name)
}
