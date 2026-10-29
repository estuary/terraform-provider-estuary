package builder

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// warnDisabledLive warns when the entry is disabled live but config doesn't set shards.disable: the apply keeps it
// disabled without shards, and enables it with shards set, which plans show only as a changed shards value.
func (entryResource *entryResource[D, S]) warnDisabledLive(config tftypes.Value, state tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var name string = entryResource.catalogType.name

	shards := blockAttribute(config, shardsAttribute, tftypes.DynamicPseudoType)
	configured := blockAttribute(shards, disableAttribute, tftypes.Bool)
	disabled, _ := known[bool](blockAttribute(blockAttribute(state, shardsAttribute, tftypes.DynamicPseudoType), disableAttribute, tftypes.Bool))

	if !configured.IsKnown() || !configured.IsNull() || !disabled {
		return diagnostics
	}

	var detail string = "The " + name + " is disabled in Estuary, for example from the dashboard, and the configuration doesn't " +
		"set shards, so the apply keeps it disabled. Set shards = { disable = false } to enable it, or shards = { disable = true } " +
		"to manage it as disabled."

	if !shards.IsNull() {
		detail = "The " + name + " is disabled in Estuary, for example from the dashboard, and the configured shards don't set " +
			"disable, so the apply enables it. Add disable = true to shards to keep it disabled."
	}

	diagnostics.AddAttributeWarning(path.Root(shardsAttribute), entryResource.warningSummary("Disabled outside Terraform", state), detail)

	return diagnostics
}

// plannedPublication is what publishing the plan does to the bindings: the published binding documents, the entryWide
// collections, the collections to backfill and to reset, and the backfill mode changes no backfill covers.
type plannedPublication struct {
	published    []any
	wide         []string
	backfills    []string
	resets       []string
	missingModes map[string]string
}

// newPlannedPublication returns what publishing planned binding documents over live ones does, see mergedBindings and
// backfillRequests. Each backfill mode change under modeKey is backfilled, as the connector requires, and reported in
// missingModes when neither a backfill nor a reset covers it. Plan and apply both use it, so plans show what applies do.
func newPlannedPublication(planned []any, previous []any, live []any, keepUnmanaged bool, plan tftypes.Value, prior tftypes.Value, modeKey string) (plannedPublication, error) {
	var publication plannedPublication
	var err error

	publication.published, publication.wide = mergedBindings(planned, previous, live, keepUnmanaged)

	publication.backfills, publication.resets, err = backfillRequests(plan, prior, publication.wide)

	if err != nil {
		return publication, err
	}

	var changes map[string]string = modeBackfills(publication.published, live, modeKey)

	publication.missingModes = unbackfilledModes(changes, publication.backfills, publication.resets)

	for _, collection := range slices.Sorted(maps.Keys(changes)) {
		if !slices.Contains(publication.backfills, collection) {
			publication.backfills = append(publication.backfills, collection)
		}
	}

	return publication, nil
}

// missingModesError returns the error of backfill mode changes no backfill covers, nil without any.
func missingModesError(missing map[string]string) error {
	var details []string

	for _, collection := range slices.Sorted(maps.Keys(missing)) {
		details = append(details, modeBackfillDetail(collection, missing[collection]))
	}

	if len(details) == 0 {
		return nil
	}

	return errors.New(strings.Join(details, "\n"))
}

// publicationReader returns the planned publication, computed once, reporting false when the live spec or plan
// can't be read.
type publicationReader func() (plannedPublication, bool)

// newPublicationReader returns the publicationReader of plan over state, see newPlannedPublication.
func (entryResource *entryResource[D, S]) newPublicationReader(plan tftypes.Value, state tftypes.Value, readLive liveReader) publicationReader {
	return sync.OnceValues(func() (plannedPublication, bool) {
		var publication plannedPublication

		live, found := readLive()

		if !found {
			return publication, false
		}

		_, spec, err := entryResource.codec.Spec(plan)

		if err != nil {
			return publication, false
		}

		plannedDocument, err := utils.YAMLDocument(spec)

		if err != nil {
			return publication, false
		}

		priorDocument, err := entryResource.codec.Document(state)

		if err != nil {
			return publication, false
		}

		liveDocument, err := entryResource.codec.Document(live)

		if err != nil {
			return publication, false
		}

		publication, err = newPlannedPublication(catalog.Bindings(plannedDocument), catalog.Bindings(priorDocument), catalog.Bindings(liveDocument),
			keepsUnmanagedBindings(plan), plan, state, entryResource.catalogType.backfillModeKey)

		return publication, err == nil
	})
}

// validateModeBackfills rejects backfill mode changes whose binding isn't backfilled, as the connector requires.
// Failing to read the live spec skips the check.
func (entryResource *entryResource[D, S]) validateModeBackfills(plan tftypes.Value, readPublication publicationReader) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	if entryResource.catalogType.backfillModeKey == "" {
		return diagnostics
	}

	publication, found := readPublication()

	if !found {
		return diagnostics
	}

	plannedBindings, _, _ := bindings(plan)

	for index, binding := range plannedBindings {
		if change, isMissing := publication.missingModes[bindingCollectionValue(binding)]; isMissing {
			diagnostics.AddAttributeError(path.Root(bindingsAttribute).AtListIndex(index).AtName(backfillAttribute).AtName(backfillModeAttribute),
				"Backfill mode changed without a backfill", modeBackfillDetail(bindingCollectionValue(binding), change))
		}
	}

	return diagnostics
}

// modeBackfillDetail explains the backfill a changed backfill mode requires.
func modeBackfillDetail(collection string, change string) string {
	return "backfill.mode of " + collection + " changes " + change + ". The connector requires the binding to be backfilled " +
		"when its mode changes: raise its backfill.trigger, or the capture's, in the same change."
}

// warnDataflowResets warns once about every collection the plan dataflow resets, which truncates its destination
// tables. Failing to read the live bindings skips the warning.
func (entryResource *entryResource[D, S]) warnDataflowResets(plan tftypes.Value, state tftypes.Value, readPublication publicationReader) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var resets []string
	var err error

	if raisesTrigger(plan, state) || triggersUnmanaged(plan, state) {
		publication, found := readPublication()

		if !found {
			return diagnostics
		}

		resets = publication.resets
	} else if _, resets, err = backfillRequests(plan, state, nil); err != nil {
		return diagnostics
	}

	if len(resets) == 0 {
		return entryResource.warnPendingResets(plan, state)
	}

	diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Dataflow reset", plan),
		"A raised backfill.trigger backfills these bindings with backfill.strategy = \"dataflowReset\", so the apply resets "+
			"their collections:\n\n- "+strings.Join(resets, "\n- ")+"\n\n"+
			"Their data is replaced by a fresh backfill from the source, and their inferred schemas are cleared. Every materialization "+
			"of these collections is backfilled too, truncating its destination tables, and a materialization with "+
			"on_incompatible_schema_change = \"abort\" makes the apply fail. Raising the backfill.trigger of the "+entryResource.catalogType.name+
			" covers every enabled binding, including those Terraform doesn't manage.")

	return diagnostics
}

// warnEntryBackfill warns, when the plan raises the entry backfill trigger, about every binding it backfills without
// resetting, as warnDataflowResets lists those. Failing to read the live bindings skips the warning.
func (entryResource *entryResource[D, S]) warnEntryBackfill(plan tftypes.Value, state tftypes.Value, readPublication publicationReader) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var collections []string

	if !raisesTrigger(plan, state) {
		return diagnostics
	}

	publication, found := readPublication()

	if !found {
		return diagnostics
	}

	for _, collection := range publication.wide {
		if !slices.Contains(publication.resets, collection) {
			collections = append(collections, collection)
		}
	}

	if len(collections) == 0 {
		return diagnostics
	}

	slices.Sort(collections)

	diagnostics.AddAttributeWarning(path.Root(backfillAttribute).AtName(backfillTriggerAttribute), entryResource.warningSummary("Backfill of every binding", plan),
		"Raising the backfill.trigger of the "+entryResource.catalogType.name+" backfills every enabled binding, including those "+
			"Terraform doesn't manage:\n\n- "+strings.Join(collections, "\n- ")+"\n\n"+
			"Disabled bindings are skipped. To backfill only some bindings, keep this trigger at its current value and raise "+
			"their own backfill.trigger instead.")

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
		"These bindings resolve to backfill.strategy = \"dataflowReset\":\n\n- "+strings.Join(pending, "\n- ")+"\n\n"+
			"This apply doesn't reset them, but the next time their backfill.trigger, or the "+entryResource.catalogType.name+"'s, is raised, "+
			"it will: their collections will be replaced by a fresh backfill from the source, and every materialization of them "+
			"will truncate its destination tables. To backfill a binding without a reset, set backfill.strategy = \"incremental\" on it.")

	return diagnostics
}

// addedBindings describes, for warnings, the bindings Estuary adds to an entry without Terraform.
type addedBindings struct {
	setting   path.Path // attribute making Estuary add them
	generator path.Path // attribute letting Estuary add or change bindings, see catalog.BindingsGenerator
	summary   string    // summary of the warning when the entry starts adding them
	origin    string    // what each added binding comes from
	growth    string    // when Estuary adds more of them
	limits    string    // settings added bindings don't get, empty when none
	readding  string    // when a removed binding may be added back
	stopping  string    // what a disabled binding stops doing
}

// spec returns the spec of value, reporting false for null values and values it can't decode yet.
func (entryResource *entryResource[D, S]) spec(value tftypes.Value) (S, bool) {
	var spec S

	if value.IsNull() {
		return spec, false
	}

	value, err := withoutTargets(value)

	if err != nil {
		return spec, false
	}

	_, spec, err = entryResource.codec.Spec(value)

	return spec, err == nil
}

// addsBindings reports whether Estuary adds bindings to the entry of value. It skips values it can't decode yet.
func (entryResource *entryResource[D, S]) addsBindings(value tftypes.Value) bool {
	spec, decoded := entryResource.spec(value)

	return decoded && addsNewBindings(spec)
}

// validateKeepUnmanagedBindings rejects keep_unmanaged_bindings = false while Estuary may add bindings, as each apply
// would remove them. It skips plans it can't decode yet.
func (entryResource *entryResource[D, S]) validateKeepUnmanagedBindings(plan tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var added addedBindings = entryResource.catalogType.added
	var generator string = added.generator.String()

	spec, decoded := entryResource.spec(plan)

	if keepsUnmanagedBindings(plan) || !decoded || !generatesBindings(spec) {
		return diagnostics
	}

	diagnostics.AddAttributeError(path.Root(keepUnmanagedBindingsAttribute),
		entryResource.warningSummary("keep_unmanaged_bindings = false can't be used with "+generator, plan),
		"With "+generator+" set, Estuary adds a binding for every "+added.origin+" that isn't declared in bindings. "+
			"keep_unmanaged_bindings = false makes Terraform remove those bindings, so each apply would remove what Estuary adds back.\n\n"+
			"To fix it, either:\n"+
			"- remove keep_unmanaged_bindings = false, so Terraform keeps the bindings Estuary adds (the default), or\n"+
			"- remove "+generator+", so Estuary stops adding bindings and Terraform manages them all.")

	return diagnostics
}

// warnUnmanagedBindingsRemoval warns about the undeclared live bindings the apply removes when keep_unmanaged_bindings
// turns false, as state doesn't hold them for the plan to show. It skips unknown collections and unreadable live specs.
func (entryResource *entryResource[D, S]) warnUnmanagedBindingsRemoval(plan tftypes.Value, state tftypes.Value, readLive liveReader) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	if keepsUnmanagedBindings(plan) || !keepsUnmanagedBindings(state) {
		return diagnostics
	}

	declared, isKnown := bindingCollections(plan, state)

	if !isKnown {
		return diagnostics
	}

	live, found := readLive()

	if !found {
		return diagnostics
	}

	removed, err := undeclaredCollections(live, declared)

	if err == nil && len(removed) > 0 {
		diagnostics.AddAttributeWarning(path.Root(keepUnmanagedBindingsAttribute), entryResource.warningSummary("Unmanaged bindings removed", plan),
			"With keep_unmanaged_bindings = false, Terraform manages every binding, so the apply removes these live bindings the "+
				"configuration doesn't declare, which the plan can't show:\n\n- "+strings.Join(removed, "\n- ")+
				"\n\nTo keep one, declare it in bindings.")
	}

	return diagnostics
}

// warnAddedBindings warns when the plan makes Estuary start adding bindings Terraform won't manage.
func (entryResource *entryResource[D, S]) warnAddedBindings(plan tftypes.Value, state tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var added addedBindings = entryResource.catalogType.added

	if !entryResource.addsBindings(plan) || entryResource.addsBindings(state) {
		return diagnostics
	}

	var detail string = "With " + added.setting.String() + " set, Estuary adds a binding for every " + added.origin + " that isn't " +
		"declared in bindings, as the dashboard does, and keeps adding them as " + added.growth + ".\n\n" +
		"Terraform doesn't manage these bindings: plans don't show them and applies keep them. To manage one, declare it in bindings."

	if added.limits != "" {
		detail += "\n\n" + added.limits
	}

	diagnostics.AddAttributeWarning(added.setting, entryResource.warningSummary(added.summary, plan), detail)

	return diagnostics
}

// warnRemovedBindings warns about removed bindings that Estuary may add back. It skips unknown collections.
func (entryResource *entryResource[D, S]) warnRemovedBindings(plan tftypes.Value, state tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var added addedBindings = entryResource.catalogType.added

	if !entryResource.addsBindings(plan) {
		return diagnostics
	}

	planned, isKnown := bindingCollections(plan)

	if !isKnown {
		return diagnostics
	}

	removed, err := undeclaredCollections(state, planned)

	if err == nil && len(removed) > 0 {
		diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Removed bindings may be added back", plan),
			"These bindings are removed from the "+entryResource.catalogType.name+", but with "+added.setting.String()+" set, "+
				added.readding+":\n\n- "+strings.Join(removed, "\n- ")+
				"\n\nTo stop "+added.stopping+" one, keep its binding and set disable = true instead.")
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
		if collection := bindingCollectionValue(binding); collection != "" && !managed[collection] {
			candidates[index] = collection
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
		collection, isCandidate := candidates[index]
		liveBinding, isLive := liveByCollection[collection]

		if !isCandidate || !isLive {
			continue
		}

		changes, err := entryResource.codec.BindingChanges(bindingsPath.WithElementKeyInt(index), binding, liveBinding)

		if err != nil {
			continue
		}

		var line string = fmt.Sprintf("- %s (bindings[%d]): no field changes", collection, index)

		if len(changes) > 0 {
			line = fmt.Sprintf("- %s (bindings[%d]):\n    %s", collection, index, strings.Join(changes, "\n    "))
		}

		takenOver = append(takenOver, line)
	}

	if len(takenOver) > 0 {
		diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Existing bindings taken over", plan),
			"These bindings already exist in the "+entryResource.catalogType.name+". Instead of adding them, Terraform takes them over and "+
				"updates them in place, changing only the fields listed below compared with the live binding:\n\n"+strings.Join(takenOver, "\n")+
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
		diagnostics.AddError("Planning "+entryResource.catalogType.name+" secrets", err.Error())
		return plan, diagnostics
	}

	for _, secret := range changed {
		diagnostics.AddAttributeWarning(attributePath(secret), entryResource.warningSummary("Secret changed: "+attributePath(secret).String(), plan),
			attributePath(secret).String()+" changed, so the "+entryResource.catalogType.name+" is published with the new value.")
	}

	return planned, diagnostics
}

// warningSummary appends the catalog name to summary, as Terraform merges same-summary warnings across resources.
func (entryResource *entryResource[D, S]) warningSummary(summary string, plan tftypes.Value) string {
	name, isKnown := known[string](blockAttribute(plan, nameAttribute, tftypes.String))

	if !isKnown {
		return summary
	}

	return summary + " in " + entryResource.catalogName(name)
}
