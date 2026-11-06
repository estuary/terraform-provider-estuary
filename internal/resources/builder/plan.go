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

	var detail string = "The " + name + " was disabled outside Terraform, for example from the dashboard. The configuration " +
		"doesn't set shards, so the apply keeps it disabled. Set shards = { disable = false } to enable it, or " +
		"shards = { disable = true } to keep managing it as disabled."

	if !shards.IsNull() {
		detail = "The " + name + " was disabled outside Terraform, for example from the dashboard. The configured shards don't " +
			"set disable, so the apply enables it again. Add disable = true to shards to keep it disabled."
	}

	diagnostics.AddAttributeWarning(path.Root(shardsAttribute), entryResource.warningSummary("Disabled outside Terraform", state), detail)

	return diagnostics
}

// plannedPublication is what publishing the plan does to the bindings: the published bindings, the indexes of the
// entryWide ones and of those to backfill, the collections to reset, and the binding changes requiring a backfill no
// backfill covers.
type plannedPublication struct {
	published []publishedBinding
	wide      []int
	backfills []int
	resets    []string
	missing   []missingBackfill
}

// newPlannedPublication returns what publishing planned binding documents over live ones does, paired as pairing pairs
// their bindings, see publishedBindings and backfillRequests. Each backfill mode change is backfilled, as the connector
// requires, unless a reset of its collection covers it. Changes the connector only accepts along with a backfill,
// backfill mode changes and switches from delta to standard updates, are reported in missing when no backfill covers
// them. Plan and apply both use it, so plans show what applies do.
func newPlannedPublication(planned []any, live []any, keepUnmanaged bool, pairing bindingPairing, catalogType catalogType) (plannedPublication, error) {
	var publication plannedPublication

	paired, found := pairing.readLive()

	if !found {
		return publication, errors.New("the live bindings can't be read")
	}

	published, err := paired.publishedBindings(planned, live, keepUnmanaged)

	if err != nil {
		return publication, err
	}

	publication.published = published
	publication.wide = entryWide(publication.published)
	publication.backfills, publication.resets = pairing.backfillRequests(publication.published, publication.wide)

	var changes map[int]string = modeBackfills(publication.published, live, catalogType.backfillModeKey)

	publication.missing = append(missingModeBackfills(publication.published, changes, publication.backfills, publication.resets),
		missingStandardUpdatesBackfills(publication.published, live, catalogType.deltaUpdatesKey, publication.backfills)...)

	for index := range publication.missing {
		if pairing.recordsTriggers {
			publication.missing[index].detail += " On the first apply after an import, triggers only record their starting " +
				"value: apply once without this change, then set or raise the trigger along with it."
		}
	}

	for _, index := range slices.Sorted(maps.Keys(changes)) {
		if !slices.Contains(publication.backfills, index) && !slices.Contains(publication.resets, catalog.BindingCollection(publication.published[index].document)) {
			publication.backfills = append(publication.backfills, index)
		}
	}

	return publication, nil
}

// publicationReader returns the planned publication, computed once, reporting false when the live spec or plan
// can't be read.
type publicationReader func() (plannedPublication, bool)

// newPublicationReader returns the publicationReader of the paired plan over state, see newPlannedPublication.
func (entryResource *entryResource[D, S]) newPublicationReader(pairing bindingPairing) publicationReader {
	return sync.OnceValues(func() (plannedPublication, bool) {
		var publication plannedPublication

		live, found := pairing.readLive()

		if !found {
			return publication, false
		}

		_, spec, err := entryResource.codec.Spec(pairing.plan)

		if err != nil {
			return publication, false
		}

		plannedDocument, err := utils.YAMLDocument(spec)

		if err != nil {
			return publication, false
		}

		liveDocument, err := entryResource.codec.Document(live.value)

		if err != nil {
			return publication, false
		}

		publication, err = newPlannedPublication(catalog.Bindings(plannedDocument), catalog.Bindings(liveDocument), keepsUnmanagedBindings(pairing.plan),
			pairing, entryResource.catalogType)

		return publication, err == nil
	})
}

// validateMissingBackfills rejects the binding changes the connector only accepts along with a backfill, see
// newPlannedPublication, whose binding isn't backfilled. Failing to read the live spec skips the check.
func (entryResource *entryResource[D, S]) validateMissingBackfills(readPublication publicationReader) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	if entryResource.catalogType.backfillModeKey == "" && entryResource.catalogType.deltaUpdatesKey == "" {
		return diagnostics
	}

	publication, found := readPublication()

	if !found {
		return diagnostics
	}

	for _, change := range publication.missing {
		var attributePath path.Path = path.Root(bindingsAttribute).AtListIndex(change.binding)

		for _, name := range change.attribute {
			attributePath = attributePath.AtName(name)
		}

		diagnostics.AddAttributeError(attributePath, change.summary, change.detail)
	}

	return diagnostics
}

// warnDataflowResets warns once about every collection the plan dataflow resets, which truncates its destination
// tables. Failing to read the live bindings skips the warning.
func (entryResource *entryResource[D, S]) warnDataflowResets(pairing bindingPairing, readPublication publicationReader) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var resets []string
	var plan tftypes.Value = pairing.plan

	if pairing.raisesTriggers() {
		publication, found := readPublication()

		if !found {
			return diagnostics
		}

		resets = publication.resets
	}

	if len(resets) == 0 {
		return entryResource.warnPendingResets(pairing)
	}

	diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Dataflow reset", plan),
		entryResource.resetCause(pairing)+"\n\n- "+strings.Join(resets, "\n- ")+"\n\n"+
			"Each collection's data is replaced by a fresh backfill from the source, and its inferred schema is cleared. Every "+
			"materialization of these collections is backfilled too, which truncates its destination tables, and the apply fails "+
			"if one has on_incompatible_schema_change = \"abort\". Derivations reading these collections are validated in the "+
			"same publication, so the apply also fails if one relies on fields that only the inferred schemas had.")

	return diagnostics
}

// resetCause explains which backfill trigger resets the collections: the entry one covers every enabled binding,
// binding ones only their own.
func (entryResource *entryResource[D, S]) resetCause(pairing bindingPairing) string {
	if pairing.raises(pairing.plan, pairing.state) {
		return "Setting or raising the backfill.trigger of the " + entryResource.catalogType.name + " backfills every enabled " +
			"binding, including those Terraform doesn't manage. These bindings use backfill.strategy = \"dataflowReset\", so the " +
			"apply resets their collections:"
	}

	return "These bindings have a set or raised backfill.trigger and use backfill.strategy = \"dataflowReset\", so the apply " +
		"resets their collections. The other bindings of the " + entryResource.catalogType.name + " aren't backfilled:"
}

// warnBindingBackfills warns, when the plan sets or raises binding backfill triggers but not the entry one, about every
// binding it backfills without resetting, as warnDataflowResets lists those. Failing to read the live bindings skips
// the warning.
func (entryResource *entryResource[D, S]) warnBindingBackfills(pairing bindingPairing, readPublication publicationReader) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	if pairing.raises(pairing.plan, pairing.state) || !pairing.raisesTriggers() {
		return diagnostics
	}

	publication, found := readPublication()

	if !found || len(publication.backfills) == 0 {
		return diagnostics
	}

	diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Backfill of bindings", pairing.plan),
		"These bindings have a set or raised backfill.trigger, so the apply backfills them:\n\n- "+
			strings.Join(bindingCollections(publication.published, publication.backfills), "\n- ")+"\n\n"+
			"The other bindings of the "+entryResource.catalogType.name+" aren't backfilled. A trigger set on a binding Terraform "+
			"already manages backfills it: to keep a binding as it is, leave its trigger unset.")

	return diagnostics
}

// warnEntryBackfill warns, when the plan sets or raises the entry backfill trigger, about every binding it backfills
// without resetting, as warnDataflowResets lists those. Failing to read the live bindings skips the warning.
func (entryResource *entryResource[D, S]) warnEntryBackfill(pairing bindingPairing, readPublication publicationReader) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var plan tftypes.Value = pairing.plan

	if !pairing.raises(plan, pairing.state) {
		return diagnostics
	}

	publication, found := readPublication()

	if !found {
		return diagnostics
	}

	var collections []string = slices.DeleteFunc(bindingCollections(publication.published, publication.wide), func(collection string) bool {
		return slices.Contains(publication.resets, collection)
	})

	if len(collections) == 0 {
		return diagnostics
	}

	diagnostics.AddAttributeWarning(path.Root(backfillAttribute).AtName(backfillTriggerAttribute), entryResource.warningSummary("Backfill of every binding", plan),
		"Setting or raising the backfill.trigger of the "+entryResource.catalogType.name+" backfills every enabled binding, "+
			"including those Terraform doesn't manage:\n\n- "+strings.Join(collections, "\n- ")+"\n\n"+
			"Disabled bindings are skipped. To backfill only some bindings, leave this trigger as it was and set or raise "+
			"their own backfill.trigger instead.")

	return diagnostics
}

// warnPendingResets warns, when the plan changes a backfill block without resetting, about the bindings whose strategy
// resolves to dataflowReset, so a later raise of a trigger doesn't reset them unnoticed.
func (entryResource *entryResource[D, S]) warnPendingResets(pairing bindingPairing) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var plan tftypes.Value = pairing.plan

	if !pairing.backfillChanged() {
		return diagnostics
	}

	pending, err := pendingResets(plan)

	if err != nil || len(pending) == 0 {
		return diagnostics
	}

	diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Bindings set to dataflow reset", plan),
		"These bindings use backfill.strategy = \"dataflowReset\", set on them or on the "+entryResource.catalogType.name+":\n\n- "+
			strings.Join(pending, "\n- ")+"\n\n"+
			"This apply doesn't reset them. But the next time their backfill.trigger or the "+entryResource.catalogType.name+"'s is set or raised, "+
			"their collections are replaced by a fresh backfill from the source, and every materialization of them truncates its "+
			"destination tables. To backfill a binding without a reset, set backfill.strategy = \"incremental\" on it.")

	return diagnostics
}

// addedBindings describes, for warnings, the bindings Estuary adds to an entry without Terraform.
type addedBindings struct {
	setting   path.Path
	generator path.Path
	summary   string
	origin    string
	growth    string
	limits    string
	readding  string
	stopping  string
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
func (entryResource *entryResource[D, S]) warnUnmanagedBindingsRemoval(pairing bindingPairing) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var plan tftypes.Value = pairing.plan

	if keepsUnmanagedBindings(plan) || !keepsUnmanagedBindings(pairing.state) {
		return diagnostics
	}

	removed, isKnown := pairing.unmanagedBindings()

	if isKnown && len(removed) > 0 {
		diagnostics.AddAttributeWarning(path.Root(keepUnmanagedBindingsAttribute), entryResource.warningSummary("Unmanaged bindings removed", plan),
			"With keep_unmanaged_bindings = false, Terraform manages every binding, so the apply removes these bindings, which "+
				"exist in Estuary but aren't declared in the configuration. The plan can't show their removal:\n\n- "+strings.Join(removed, "\n- ")+
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

// warnNamingChange warns that a target_naming change, as in the dashboard, only names the bindings added from now on.
func (entryResource *entryResource[D, S]) warnNamingChange(plan tftypes.Value, state tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	previous, err := attributeValue(state, targetNamingPath)

	if err != nil {
		return diagnostics
	}

	current, err := attributeValue(plan, targetNamingPath)

	if err != nil || !current.IsFullyKnown() || current.Equal(previous) {
		return diagnostics
	}

	diagnostics.AddAttributeWarning(path.Root(targetNamingAttribute), entryResource.warningSummary("Target naming change only names new bindings", plan),
		"As in the dashboard, the new target_naming only names the bindings added from now on: those the source capture "+
			"adds and those declared without their own schema or table. Existing bindings keep theirs, so the plan shows no "+
			"change to them.\n\nTo move an existing binding, declare it in bindings with its resource schema or table set to "+
			"the new name. The connector then materializes it into a new table and leaves the old table in place.")

	return diagnostics
}

// warnMovedTables warns about bindings the plan moves to another table, which the connector backfills from scratch.
func (entryResource *entryResource[D, S]) warnMovedTables(pairing bindingPairing) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var plan tftypes.Value = pairing.plan

	if entryResource.catalogType.movedTables == nil {
		return diagnostics
	}

	var moved []string = entryResource.catalogType.movedTables(pairing)

	if len(moved) > 0 {
		diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Bindings materialize into new tables", plan),
			"These bindings change their schema or table, so the connector materializes each into a new table:\n\n- "+strings.Join(moved, "\n- ")+
				"\n\nThe connector writes new data to each new table and leaves the old table in place, no longer updated. "+
				"Depending on the connector, the data already materialized may not be copied over: to fill a new table from the "+
				"start of its collection, raise the binding's backfill.trigger. For large collections, this can take a long time and "+
				"cost warehouse credits.\n\nIf a new table already exists in the destination, the apply fails. Drop the table "+
				"first, or ask Estuary support whether the allow_existing_tables_for_new_bindings feature flag suits your case.")
	}

	return diagnostics
}

// warnRemovedBindings warns about removed bindings that Estuary may add back. It skips unknown collections.
func (entryResource *entryResource[D, S]) warnRemovedBindings(pairing bindingPairing) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var added addedBindings = entryResource.catalogType.added
	var plan tftypes.Value = pairing.plan

	if !entryResource.addsBindings(plan) {
		return diagnostics
	}

	removed, isKnown := pairing.removedBindings()

	if isKnown && len(removed) > 0 {
		diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Removed bindings may be added back", plan),
			"These bindings are removed from the "+entryResource.catalogType.name+", but with "+added.setting.String()+" set, "+
				added.readding+":\n\n- "+strings.Join(removed, "\n- ")+
				"\n\nTo stop "+added.stopping+" one, keep its binding and set disable = true instead.")
	}

	return diagnostics
}

// warnTakenOverBindings warns about the unmanaged live bindings the apply takes over and updates in place, listing the
// changed fields: those a planned binding has the collection and resource path of, whether new or moved from the
// resource it was published on, as plans show neither as a takeover. Failing to read the live spec skips the warning.
func (entryResource *entryResource[D, S]) warnTakenOverBindings(pairing bindingPairing) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var plan tftypes.Value = pairing.plan
	var takenOver []string

	if !pairing.mayTakeOver() {
		return diagnostics
	}

	live, found := pairing.readLive()

	if !found {
		return diagnostics
	}

	for index, binding := range pairing.planned.values {
		var collection string = bindingCollectionValue(binding)
		var liveIndex int = live.lives[index]

		if liveIndex == -1 || live.managed[liveIndex] != -1 {
			continue
		}

		changes, err := entryResource.codec.BindingChanges(bindingsPath.WithElementKeyInt(index), binding, live.bindings.values[liveIndex])

		if err != nil {
			continue
		}

		var line string = fmt.Sprintf("- %s (bindings[%d]): no field changes", collection, index)

		if len(changes) > 0 {
			line = fmt.Sprintf("- %s (bindings[%d]):\n    %s", collection, index, strings.Join(changes, "\n    "))
		}

		if pairing.priors[index] != -1 {
			line += "\n    The binding Terraform managed on its previous resource is removed."
		}

		takenOver = append(takenOver, line)
	}

	if len(takenOver) > 0 {
		diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Existing bindings taken over", plan),
			"These bindings have the collection and resource of bindings that already exist in the "+entryResource.catalogType.name+
				" but that Terraform doesn't manage, such as ones added from the dashboard. Instead of adding them, Terraform takes them "+
				"over and updates them in place, changing only the fields listed below compared with the live binding:\n\n"+
				strings.Join(takenOver, "\n")+"\n\nThis is expected the first time Terraform manages an existing binding, and no action "+
				"is needed. A binding moved onto the resource of an existing one replaces it: to keep it, move the binding elsewhere.")
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
