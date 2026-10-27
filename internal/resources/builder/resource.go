package builder

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v3"
)

// publication is what publishing an entry does besides its spec: discover adds discovered bindings, matched
// through identifier, and resets lists the collections it dataflow resets.
type publication struct {
	discover   bool
	identifier catalog.BindingsIdentifier
	resets     []string
}

// publisher publishes a catalog entry as its YAML document.
type publisher func(ctx context.Context, backend catalog.Backend, name string, document map[string]any, publication publication) error

type entryResource[D Document[D, S], S any] struct {
	connector Connector
	entity    string
	codec     *Codec[D, S]
	err       error
	publish   publisher
	session   *catalog.Session
}

// Compile-time checks that entryResource implements the framework resource interfaces.
var _ resource.ResourceWithConfigure = &entryResource[captureDocument[struct{}, struct{}], CaptureSpec[struct{}, struct{}]]{}
var _ resource.ResourceWithImportState = &entryResource[captureDocument[struct{}, struct{}], CaptureSpec[struct{}, struct{}]]{}
var _ resource.ResourceWithModifyPlan = &entryResource[captureDocument[struct{}, struct{}], CaptureSpec[struct{}, struct{}]]{}

func newResource[D Document[D, S], S any](connector Connector, entity string, publish publisher) func() resource.Resource {
	return func() resource.Resource {
		codec, err := NewCodec[D, S](connector)

		return &entryResource[D, S]{
			connector: connector,
			entity:    entity,
			codec:     codec,
			err:       err,
			publish:   publish,
		}
	}
}

func (entryResource *entryResource[D, S]) Metadata(_ context.Context, metadataReq resource.MetadataRequest, metadataResp *resource.MetadataResponse) {
	metadataResp.TypeName = metadataReq.ProviderTypeName + "_" + entryResource.connector.TypeName
}

func (entryResource *entryResource[D, S]) Schema(_ context.Context, _ resource.SchemaRequest, schemaResp *resource.SchemaResponse) {
	if entryResource.err != nil {
		schemaResp.Diagnostics.AddError("Invalid connector schema", entryResource.err.Error())
		return
	}

	schemaResp.Schema = entryResource.codec.Schema()
}

func (entryResource *entryResource[D, S]) Configure(_ context.Context, configureReq resource.ConfigureRequest, configureResp *resource.ConfigureResponse) {
	if configureReq.ProviderData == nil {
		return
	}

	session, ok := configureReq.ProviderData.(*catalog.Session)

	if !ok {
		configureResp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *catalog.Session, got %T", configureReq.ProviderData))
		return
	}

	entryResource.session = session
}

// ModifyPlan plans secret hashes, binding targets and names, and validates backfill triggers. It replaces the entry
// only when its catalog name changes, with or without tenant and connector name being the same entry.
func (entryResource *entryResource[D, S]) ModifyPlan(ctx context.Context, modifyPlanReq resource.ModifyPlanRequest, modifyPlanResp *resource.ModifyPlanResponse) {
	var stateName types.String
	var planName types.String
	var readLive liveReader = entryResource.newLiveReader(ctx, modifyPlanReq.State.Raw)

	if modifyPlanReq.Plan.Raw.IsNull() {
		return
	}

	planned, diagnostics := entryResource.planSecretsHash(modifyPlanReq.Plan.Raw, modifyPlanReq.Config.Raw, modifyPlanReq.State.Raw)

	modifyPlanResp.Diagnostics.Append(diagnostics...)

	planned, err := entryResource.planTargets(ctx, planned, modifyPlanReq.Config.Raw, modifyPlanReq.State.Raw, readLive)

	if err == nil {
		planned, err = withBindingNames(planned)
	}

	if err == nil {
		planned, err = planShards(planned, modifyPlanReq.Config.Raw, modifyPlanReq.State.Raw)
	}

	if err != nil {
		modifyPlanResp.Diagnostics.AddError("Planning "+entryResource.entity+" bindings", err.Error())
	}

	modifyPlanResp.Diagnostics.Append(validateBackfillTriggers(planned, modifyPlanReq.State.Raw)...)

	modifyPlanResp.Plan.Raw = planned

	if modifyPlanReq.State.Raw.IsNull() && !modifyPlanResp.Diagnostics.HasError() {
		modifyPlanResp.Diagnostics.Append(entryResource.warnDiscoveredBindings(planned)...)
	}

	if modifyPlanReq.State.Raw.IsNull() || modifyPlanResp.Diagnostics.HasError() {
		return
	}

	modifyPlanResp.Diagnostics.Append(modifyPlanReq.State.GetAttribute(ctx, path.Root("name"), &stateName)...)
	modifyPlanResp.Diagnostics.Append(modifyPlanReq.Plan.GetAttribute(ctx, path.Root("name"), &planName)...)

	if modifyPlanResp.Diagnostics.HasError() {
		return
	}

	if planName.IsUnknown() || entryResource.catalogName(planName.ValueString()) != entryResource.catalogName(stateName.ValueString()) {
		modifyPlanResp.RequiresReplace = append(modifyPlanResp.RequiresReplace, path.Root("name"))
		return
	}

	modifyPlanResp.Diagnostics.Append(entryResource.warnTakenOverBindings(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw, readLive)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnRemovedBindings(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnDataflowResets(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw, readLive)...)
	modifyPlanResp.Diagnostics.Append(entryResource.validateModeBackfills(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw, readLive)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnDisabledLive(modifyPlanReq.Config.Raw, modifyPlanReq.State.Raw)...)
}

// warnDisabledLive warns when state is disabled live but config doesn't set shards.disable, which plans don't show.
func (entryResource *entryResource[D, S]) warnDisabledLive(config tftypes.Value, state tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var disabled bool

	configured := bindingAttribute(bindingAttribute(config, shardsAttribute), shardsDisableKey)
	live := bindingAttribute(bindingAttribute(state, shardsAttribute), shardsDisableKey)

	if !configured.IsKnown() || !configured.IsNull() || !live.IsKnown() || live.IsNull() || live.As(&disabled) != nil || !disabled {
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

	live, found := readLive()

	if !found {
		return diagnostics
	}

	plannedDocument, err := entryResource.codec.Document(plan)

	if err != nil {
		return diagnostics
	}

	liveDocument, err := entryResource.codec.Document(live)

	if err != nil {
		return diagnostics
	}

	_, spec, err := entryResource.codec.Spec(plan)

	if err != nil {
		return diagnostics
	}

	wide, err := captureWide(plan, state, live, generatesBindings(spec))

	if err != nil {
		return diagnostics
	}

	liveBindings, _ := liveDocument[bindingsAttribute].([]any)

	missing, err := unbackfilledModes(plannedDocument[bindingsAttribute], liveBindings, holder.resourceBackfillMode(), plan, state, wide)

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

	if raisesTrigger(plan, state) || triggersUnmanaged(plan, state) {
		live, found := readLive()

		if !found {
			return diagnostics
		}

		_, spec, err := entryResource.codec.Spec(plan)

		if err != nil {
			return diagnostics
		}

		wide, err = captureWide(plan, state, live, generatesBindings(spec))

		if err != nil {
			return diagnostics
		}
	}

	_, resets, err := backfillRequests(plan, state, wide)

	if err != nil || len(resets) == 0 {
		return diagnostics
	}

	diagnostics.AddAttributeWarning(path.Root(bindingsAttribute), entryResource.warningSummary("Dataflow reset", plan),
		"A raised backfill.trigger resolves to backfill.strategy = \"dataflowReset\" for these bindings, so the apply resets "+
			"their collections:\n\n- "+strings.Join(resets, "\n- ")+"\n\n"+
			"Their data is replaced by a fresh backfill from the source and their inferred schemas are cleared. Every materialization "+
			"of them backfills, truncating its destination tables, and a materialization with on_incompatible_schema_change = \"abort\" "+
			"fails the apply. Raising the backfill.trigger of the "+entryResource.entity+" covers every binding, those Terraform "+
			"doesn't manage included.")

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

func (entryResource *entryResource[D, S]) Create(ctx context.Context, createReq resource.CreateRequest, createResp *resource.CreateResponse) {
	createResp.Diagnostics.Append(entryResource.apply(ctx, createReq.Plan, createReq.Config, nil, &createResp.State)...)
}

func (entryResource *entryResource[D, S]) Read(ctx context.Context, readReq resource.ReadRequest, readResp *resource.ReadResponse) {
	var name types.String

	readResp.Diagnostics.Append(readReq.State.GetAttribute(ctx, path.Root("name"), &name)...)

	if readResp.Diagnostics.HasError() {
		return
	}

	readResp.Diagnostics.Append(entryResource.readLive(ctx, name.ValueString(), entryResource.catalogName(name.ValueString()), readReq, readResp)...)
}

// readLive refreshes state from the live spec, removing deleted entries. An undecodable live spec keeps state with
// a warning, rather than failing every plan.
func (entryResource *entryResource[D, S]) readLive(ctx context.Context, name string, catalogName string, readReq resource.ReadRequest, readResp *resource.ReadResponse) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	sealed, err := entryResource.session.Backend.ReadSpec(ctx, catalogName)

	if err != nil {
		diagnostics.AddError("Reading "+entryResource.entity+" "+catalogName, err.Error())
		return diagnostics
	}

	if sealed == nil {
		readResp.State.RemoveResource(ctx)
		return diagnostics
	}

	refreshed, err := entryResource.refreshedState(name, readReq.State.Raw, sealed)

	if err != nil {
		diagnostics.AddWarning("Not refreshing "+entryResource.entity+" "+catalogName,
			err.Error()+"\n\nThe plan doesn't show the changes made outside Terraform.")
		return diagnostics
	}

	readResp.State.Raw = refreshed

	return diagnostics
}

// refreshedState returns state refreshed from the sealed live spec. Live bindings state doesn't hold stay unmanaged
// when the control plane may add bindings.
func (entryResource *entryResource[D, S]) refreshedState(name string, state tftypes.Value, sealed map[string]any) (tftypes.Value, error) {
	live, err := entryResource.liveValue(name, sealed)

	if err != nil {
		return state, err
	}

	_, stateSpec, err := entryResource.codec.Spec(state)

	if err != nil {
		return state, err
	}

	return entryResource.codec.Refresh(state, live, !generatesBindings(stateSpec))
}

func (entryResource *entryResource[D, S]) Update(ctx context.Context, updateReq resource.UpdateRequest, updateResp *resource.UpdateResponse) {
	updateResp.Diagnostics.Append(entryResource.apply(ctx, updateReq.Plan, updateReq.Config, &updateReq.State, &updateResp.State)...)
}

func (entryResource *entryResource[D, S]) Delete(ctx context.Context, deleteReq resource.DeleteRequest, deleteResp *resource.DeleteResponse) {
	var name types.String

	deleteResp.Diagnostics.Append(deleteReq.State.GetAttribute(ctx, path.Root("name"), &name)...)

	if deleteResp.Diagnostics.HasError() {
		return
	}

	var catalogName string = entryResource.catalogName(name.ValueString())

	if err := entryResource.session.Backend.DeleteCatalogEntry(ctx, catalogName); err != nil {
		deleteResp.Diagnostics.AddError("Deleting "+entryResource.entity+" "+catalogName, err.Error())
	}
}

// ImportState imports by catalog name, stored with the tenant and without the connector name. Secrets are
// write-only, and bindings the control plane may add stay unmanaged.
func (entryResource *entryResource[D, S]) ImportState(ctx context.Context, importReq resource.ImportStateRequest, importResp *resource.ImportStateResponse) {
	var name string = catalog.Name(importReq.ID, entryResource.connector.ConnectorName)
	var catalogName string = entryResource.catalogName(name)

	sealed, err := entryResource.session.Backend.ReadSpec(ctx, catalogName)

	if err != nil {
		importResp.Diagnostics.AddError("Importing "+entryResource.entity+" "+catalogName, err.Error())
		return
	}

	if sealed == nil {
		importResp.Diagnostics.AddError("Importing "+entryResource.entity+" "+catalogName, "No catalog entry is named "+catalogName+".")
		return
	}

	var live map[string]any = liveDocument(sealed)

	spec, err := decode[S](live, true)

	if err != nil {
		importResp.Diagnostics.AddWarning("Importing "+entryResource.entity+" "+catalogName+" without the fields the provider doesn't manage",
			err.Error()+"\n\nThese fields are not in the imported state and the next apply removes them.")

		spec, err = decode[S](live, false)
	}

	if err == nil && generatesBindings(spec) {
		delete(live, "bindings")

		spec, err = decode[S](live, false)
	}

	if err != nil {
		importResp.Diagnostics.AddError("Importing "+entryResource.entity+" "+catalogName, err.Error())
		return
	}

	value, err := entryResource.codec.Value(name, spec)

	if err == nil {
		value, err = entryResource.codec.secrets.nullSecrets(value)
	}

	if err != nil {
		importResp.Diagnostics.AddError("Importing "+entryResource.entity+" "+catalogName, err.Error())
		return
	}

	importResp.State.Raw = value
}

// apply publishes the plan, with secrets from config, and stores it as state. Updates that leave the live spec
// unchanged, such as a trigger strategy change, publish nothing.
func (entryResource *entryResource[D, S]) apply(ctx context.Context, plan tfsdk.Plan, config tfsdk.Config, prior *tfsdk.State, state *tfsdk.State) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	planned, err := entryResource.codec.secrets.withConfigSecrets(plan.Raw, config.Raw)

	if err != nil {
		diagnostics.AddError("Converting "+entryResource.entity+" plan", err.Error())
		return diagnostics
	}

	name, spec, err := entryResource.codec.Spec(planned)

	if err != nil {
		diagnostics.AddError("Converting "+entryResource.entity+" plan", err.Error())
		return diagnostics
	}

	var catalogName string = entryResource.catalogName(name)

	document, err := utils.YAMLDocument(spec)

	if err != nil {
		diagnostics.AddError("Converting "+entryResource.entity+" plan", err.Error())
		return diagnostics
	}

	ctx = catalog.WithSecrets(ctx, entryResource.codec.secrets.values(planned)...)

	var publication publication = publication{discover: prior == nil && addsNewBindings(spec)}

	publication.identifier, _ = any(spec).(catalog.BindingsIdentifier)

	var live map[string]any

	if prior != nil {
		publication.resets, live, err = entryResource.mergeLive(ctx, catalogName, document, generatesBindings(spec), plan.Raw, prior.Raw)

		if err != nil {
			diagnostics.AddError("Merging "+entryResource.entity+" "+catalogName+" with its live spec", err.Error())
			return diagnostics
		}
	}

	if live != nil && len(publication.resets) == 0 && reflect.DeepEqual(document, live) {
		tflog.Debug(ctx, "Skipping the publication, the spec is unchanged", map[string]any{"name": catalogName})
	} else if err := entryResource.publish(ctx, entryResource.session.Backend, catalogName, document, publication); err != nil {
		diagnostics.AddError("Publishing "+entryResource.entity+" "+catalogName, err.Error())
		return diagnostics
	}

	state.Raw, err = entryResource.codec.secrets.stateHashes(plan.Raw, planned)

	if err != nil {
		diagnostics.AddError("Hashing "+entryResource.entity+" "+catalogName+" secrets", err.Error())
	}

	return diagnostics
}

// mergeLive merges the plan over the live spec, keeping live settings Terraform doesn't manage, unmanaged bindings,
// resource metadata, backfill counters and the encrypted endpoint when unchanged. It returns the resets and a
// copy of the live spec, nil when the entry doesn't exist.
func (entryResource *entryResource[D, S]) mergeLive(ctx context.Context, catalogName string, document map[string]any, generated bool, plan tftypes.Value, prior tftypes.Value) ([]string, map[string]any, error) {
	var captureWide []string
	var liveCollections []string

	live, err := entryResource.session.Backend.ReadSpec(ctx, catalogName)

	if err != nil || live == nil {
		return nil, nil, err
	}

	original, err := utils.YAMLDocument(live)

	if err != nil {
		return nil, nil, err
	}

	priorDocument, err := entryResource.codec.Document(prior)

	if err != nil {
		return nil, nil, err
	}

	for key, value := range live {
		if _, planned := document[key]; !planned && priorDocument[key] == nil {
			document[key] = value
		}
	}

	current, _ := live["bindings"].([]any)

	if generated {
		planned, _ := document["bindings"].([]any)
		previous, _ := priorDocument["bindings"].([]any)

		document["bindings"] = mergeBindings(planned, previous, current)
	}

	keepResourceMetadata(document["bindings"], current)

	for _, binding := range current {
		liveCollections = append(liveCollections, bindingCollection(binding))
	}

	published, _ := document["bindings"].([]any)

	for _, binding := range published {
		if collection := bindingCollection(binding); slices.Contains(liveCollections, collection) {
			captureWide = append(captureWide, collection)
		}
	}

	backfills, resets, err := backfillRequests(plan, prior, captureWide)

	if err != nil {
		return nil, nil, err
	}

	if holder, isHolder := any(*new(D)).(backfillModeHolder); isHolder {
		missing, err := unbackfilledModes(document["bindings"], current, holder.resourceBackfillMode(), plan, prior, captureWide)

		if err != nil {
			return nil, nil, err
		}

		var details []string

		for _, collection := range slices.Sorted(maps.Keys(missing)) {
			details = append(details, modeBackfillDetail(collection, missing[collection]))
		}

		if len(details) > 0 {
			return nil, nil, errors.New(strings.Join(details, "\n"))
		}

		for collection := range modeBackfills(document["bindings"], current, holder.resourceBackfillMode()) {
			if !slices.Contains(backfills, collection) {
				backfills = append(backfills, collection)
			}
		}
	}

	keepBackfills(document["bindings"], current, backfills)

	if entryResource.codec.secrets.changed(plan, prior) {
		tflog.Debug(ctx, "Publishing the planned endpoint, its secrets changed", map[string]any{"name": catalogName})

		return resets, original, nil
	}

	plannedDocument, err := entryResource.codec.Document(plan)

	if err != nil {
		return nil, nil, err
	}

	if !reflect.DeepEqual(plannedDocument["endpoint"], priorDocument["endpoint"]) {
		tflog.Debug(ctx, "Publishing the planned endpoint, it changed", map[string]any{"name": catalogName, "planned": endpointYAML(plannedDocument), "prior": endpointYAML(priorDocument)})

		return resets, original, nil
	}

	tflog.Debug(ctx, "Keeping the sealed live endpoint", map[string]any{"name": catalogName})

	document["endpoint"] = live["endpoint"]

	return resets, original, nil
}

// endpointYAML returns a document's endpoint as YAML for logs, safe since plans never hold write-only values.
func endpointYAML(document map[string]any) string {
	content, err := yaml.Marshal(document["endpoint"])

	if err != nil {
		return err.Error()
	}

	return string(content)
}

const shardsAttribute string = "shards"
const shardsDisableKey string = "disable"

// warningSummary appends the catalog name to summary, as Terraform merges same-summary warnings across resources.
func (entryResource *entryResource[D, S]) warningSummary(summary string, plan tftypes.Value) string {
	var name string

	value := bindingAttribute(plan, "name")

	if !value.IsKnown() || value.IsNull() || value.As(&name) != nil {
		return summary
	}

	return summary + " in " + entryResource.catalogName(name)
}

// catalogName returns the full catalog name, with tenant and connector name.
func (entryResource *entryResource[D, S]) catalogName(name string) string {
	return catalog.CatalogName(entryResource.tenant(), name, entryResource.connector.ConnectorName)
}

// tenant returns the provider tenant, empty until configured.
func (entryResource *entryResource[D, S]) tenant() string {
	if entryResource.session == nil {
		return ""
	}

	return entryResource.session.Tenant
}
