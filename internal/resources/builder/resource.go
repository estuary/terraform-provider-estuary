package builder

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// publisher publishes a catalog entry as its YAML document.
type publisher func(ctx context.Context, backend catalog.Backend, name string, document map[string]any) error

type entryResource[D Document[D, S], S any] struct {
	connector       Connector
	entity          string
	codec           *Codec[D, S]
	err             error
	publish         publisher
	session         *catalog.Session
	refreshBindings bool
}

// Register terraform inferred resources for ResourceWithConfigure, ResourceWithImportState & ResourceWithModifyPlan modes
var _ resource.ResourceWithConfigure = &entryResource[captureDocument[struct{}, struct{}], CaptureSpec[struct{}, struct{}]]{}
var _ resource.ResourceWithImportState = &entryResource[captureDocument[struct{}, struct{}], CaptureSpec[struct{}, struct{}]]{}
var _ resource.ResourceWithModifyPlan = &entryResource[captureDocument[struct{}, struct{}], CaptureSpec[struct{}, struct{}]]{}

func newResource[D Document[D, S], S any](connector Connector, entity string, publish publisher, refreshBindings bool) func() resource.Resource {
	return func() resource.Resource {
		codec, err := NewCodec[D, S](connector)

		return &entryResource[D, S]{
			connector:       connector,
			entity:          entity,
			codec:           codec,
			err:             err,
			publish:         publish,
			refreshBindings: refreshBindings,
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

// ModifyPlan plans the secrets hash, the unset binding targets and the binding names, and replaces the entry only when
// its catalog name changes, names with or without the tenant and connector name being the same entry.
func (entryResource *entryResource[D, S]) ModifyPlan(ctx context.Context, modifyPlanReq resource.ModifyPlanRequest, modifyPlanResp *resource.ModifyPlanResponse) {
	var stateName types.String
	var planName types.String
	var readLive liveReader = entryResource.newLiveReader(ctx, modifyPlanReq.State.Raw)

	if modifyPlanReq.Plan.Raw.IsNull() {
		return
	}

	modifyPlanResp.Diagnostics.Append(entryResource.planSecretsHash(modifyPlanReq, modifyPlanResp)...)

	planned, err := entryResource.planTargets(modifyPlanResp.Plan.Raw, modifyPlanReq.Config.Raw, modifyPlanReq.State.Raw, readLive)

	if err == nil {
		planned, err = withBindingNames(planned)
	}

	if err != nil {
		modifyPlanResp.Diagnostics.AddError("Planning "+entryResource.entity+" bindings", err.Error())
	}

	modifyPlanResp.Plan.Raw = planned

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
}

// warnRemovedBindings warns about each binding the plan removes while discovery adds the resources not bound, which
// may add it back. It warns about none while a planned target is unknown, as it may be the removed one.
func (entryResource *entryResource[D, S]) warnRemovedBindings(plan tftypes.Value, state tftypes.Value) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var planned map[string]bool = map[string]bool{}

	_, spec, err := entryResource.codec.Spec(plan)

	adder, isAdder := any(spec).(catalog.NewBindingsAdder)

	if err != nil || !isAdder || !adder.AddsNewBindings() {
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

		planned[bindingTarget(binding)] = true
	}

	for _, binding := range prior {
		var target string = bindingTarget(binding)

		if target == "" || planned[target] {
			continue
		}

		diagnostics.AddWarning("Removed binding may be added back",
			target+" is removed from the "+entryResource.entity+", but auto discovery adds new bindings, so the next discovery "+
				"may add it back while its resource exists.\n\nTo stop capturing it, keep the binding with disable = true instead.")
	}

	return diagnostics
}

// warnTakenOverBindings warns about each planned binding Terraform didn't manage whose target is already live, which
// plans show as added although the apply updates the live binding in place, listing the fields the apply changes.
// Failing to read the live spec only skips the warnings.
func (entryResource *entryResource[D, S]) warnTakenOverBindings(plan tftypes.Value, state tftypes.Value, readLive liveReader) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var managed map[string]bool = map[string]bool{}
	var candidates map[int]string = map[int]string{}

	if !entryResource.refreshBindings {
		return diagnostics
	}

	planned, _, err := bindings(plan)

	if err != nil {
		return diagnostics
	}

	prior, _, err := bindings(state)

	if err != nil {
		return diagnostics
	}

	for _, binding := range prior {
		managed[bindingTarget(binding)] = true
	}

	for index, binding := range planned {
		if target := bindingTarget(binding); target != "" && !managed[target] {
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

	var liveByTarget map[string]tftypes.Value = byTarget(liveBindings)

	for index, binding := range planned {
		target, isCandidate := candidates[index]
		liveBinding, isLive := liveByTarget[target]

		if !isCandidate || !isLive {
			continue
		}

		changes, err := entryResource.codec.BindingChanges(tftypes.NewAttributePath().WithAttributeName(bindingsAttribute).WithElementKeyInt(index), binding, liveBinding)

		if err != nil {
			continue
		}

		var detail string = "No field changes, Terraform only starts managing it."

		if len(changes) > 0 {
			detail = "Compared with the live binding, the apply only changes:\n  " + strings.Join(changes, "\n  ")
		}

		diagnostics.AddAttributeWarning(path.Root(bindingsAttribute).AtListIndex(index), "Existing binding taken over",
			target+" is already bound in the "+entryResource.entity+". Terraform starts managing it and updates it in place, rather than adding it.\n\n"+detail+
				"\n\nThis is expected the first time Terraform manages an existing binding, and no action is needed.")
	}

	return diagnostics
}

// planSecretsHash plans the hash beside each write-only secret, unknown when the secret changed, and warns about each
// changed secret at its attribute, as plans never show write-only values.
func (entryResource *entryResource[D, S]) planSecretsHash(modifyPlanReq resource.ModifyPlanRequest, modifyPlanResp *resource.ModifyPlanResponse) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	planned, changed, err := entryResource.codec.PlanSecretsHash(modifyPlanReq.Plan.Raw, modifyPlanReq.Config.Raw, modifyPlanReq.State.Raw)

	if err != nil {
		diagnostics.AddError("Planning "+entryResource.entity+" secrets", err.Error())
		return diagnostics
	}

	modifyPlanResp.Plan.Raw = planned

	for _, secret := range changed {
		diagnostics.AddAttributeWarning(attributePath(secret), "Secret changed",
			attributePath(secret).String()+" changed. The "+entryResource.entity+" is published with the new value.")
	}

	return diagnostics
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

	var catalogName string = entryResource.catalogName(name.ValueString())

	if entryResource.refreshBindings {
		readResp.Diagnostics.Append(entryResource.readLive(ctx, name.ValueString(), catalogName, readReq, readResp)...)
		return
	}

	entry, err := entryResource.session.Backend.ReadCatalogEntry(ctx, catalogName)

	if err != nil {
		readResp.Diagnostics.AddError("Reading "+entryResource.entity+" "+catalogName, err.Error())
		return
	}

	if entry == nil {
		readResp.State.RemoveResource(ctx)
	}
}

// readLive refreshes the state from the live spec, removing the entry when it no longer exists. A live spec the
// provider can't decode keeps the state as is with a warning, rather than failing every plan.
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
		diagnostics.AddWarning("Not refreshing "+entryResource.entity+" "+catalogName+" bindings",
			err.Error()+"\n\nThe plan doesn't show the binding changes made outside Terraform.")
		return diagnostics
	}

	readResp.State.Raw = refreshed

	return diagnostics
}

// refreshedState returns state with its bindings refreshed from the sealed live spec. Terraform manages every live
// binding unless the control plane may add some, in which case the live bindings state doesn't hold stay unmanaged.
func (entryResource *entryResource[D, S]) refreshedState(name string, state tftypes.Value, sealed map[string]any) (tftypes.Value, error) {
	live, err := entryResource.liveValue(name, sealed)

	if err != nil {
		return state, err
	}

	_, stateSpec, err := entryResource.codec.Spec(state)

	if err != nil {
		return state, err
	}

	return entryResource.codec.RefreshBindings(state, live, !generatesBindings(stateSpec))
}

// liveValue returns the Terraform value of a sealed live spec, without the secrets it can't decrypt.
func (entryResource *entryResource[D, S]) liveValue(name string, sealed map[string]any) (tftypes.Value, error) {
	liveSpec, err := entryResource.codec.LiveSpec(liveDocument(sealed), false)

	if err != nil {
		return tftypes.Value{}, err
	}

	return entryResource.codec.Value(name, liveSpec)
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

// ImportState takes the catalog name as ID, storing it with its tenant and without its connector name, along with the
// live spec. Secrets are write-only and bindings the control plane may add are left unmanaged, see mergeLiveBindings.
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

	spec, err := entryResource.codec.LiveSpec(live, true)

	if err != nil {
		importResp.Diagnostics.AddWarning("Importing "+entryResource.entity+" "+catalogName+" without the fields the provider doesn't manage",
			err.Error()+"\n\nThese fields are not in the imported state and the next apply removes them.")

		spec, err = entryResource.codec.LiveSpec(live, false)
	}

	if err == nil && generatesBindings(spec) {
		delete(live, "bindings")

		spec, err = entryResource.codec.LiveSpec(live, false)
	}

	if err != nil {
		importResp.Diagnostics.AddError("Importing "+entryResource.entity+" "+catalogName, err.Error())
		return
	}

	value, err := entryResource.codec.Value(name, spec)

	if err == nil {
		value, err = entryResource.codec.NullSecrets(value)
	}

	if err != nil {
		importResp.Diagnostics.AddError("Importing "+entryResource.entity+" "+catalogName, err.Error())
		return
	}

	importResp.State.Raw = value
}

// apply publishes the planned entry under its catalog name, its secrets taken from config, and stores the plan as its
// state with the secrets hash. Updating an entry keeps the live bindings Terraform never managed when the control plane
// may add them, see mergeBindings, and the live encrypted endpoint when it is unchanged, see sealedEndpoint.
func (entryResource *entryResource[D, S]) apply(ctx context.Context, plan tfsdk.Plan, config tfsdk.Config, prior *tfsdk.State, state *tfsdk.State) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var live map[string]any

	planned, err := entryResource.codec.WithConfigSecrets(plan.Raw, config.Raw)

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

	if prior != nil {
		live, err = entryResource.session.Backend.ReadSpec(ctx, catalogName)

		if err != nil {
			diagnostics.AddError("Reading "+entryResource.entity+" "+catalogName, err.Error())
			return diagnostics
		}
	}

	if prior != nil && generatesBindings(spec) {
		if err := entryResource.mergeLiveBindings(document, *prior, live); err != nil {
			diagnostics.AddError("Merging "+entryResource.entity+" "+catalogName+" bindings", err.Error())
			return diagnostics
		}
	}

	if prior != nil && !entryResource.codec.SecretsChanged(plan.Raw, prior.Raw) {
		endpoint, err := entryResource.sealedEndpoint(plan, *prior, live)

		if err != nil {
			diagnostics.AddError("Comparing "+entryResource.entity+" "+catalogName+" endpoint", err.Error())
			return diagnostics
		}

		if endpoint != nil {
			document["endpoint"] = endpoint
		}
	}

	if err := entryResource.publish(ctx, entryResource.session.Backend, catalogName, document); err != nil {
		diagnostics.AddError("Publishing "+entryResource.entity+" "+catalogName, err.Error())
		return diagnostics
	}

	state.Raw, err = entryResource.codec.StateSecretsHash(plan.Raw, planned)

	if err != nil {
		diagnostics.AddError("Hashing "+entryResource.entity+" "+catalogName+" secrets", err.Error())
	}

	return diagnostics
}

// sealedEndpoint returns the live endpoint, its configs encrypted by Estuary, when the planned endpoint is the prior
// one and nobody changed it since, so publishing it keeps the stored secrets. It returns nil otherwise.
func (entryResource *entryResource[D, S]) sealedEndpoint(plan tfsdk.Plan, prior tfsdk.State, live map[string]any) (any, error) {
	if live == nil {
		return nil, nil
	}

	_, plannedSpec, err := entryResource.codec.Spec(plan.Raw)

	if err != nil {
		return nil, err
	}

	_, priorSpec, err := entryResource.codec.Spec(prior.Raw)

	if err != nil {
		return nil, err
	}

	liveSpec, err := entryResource.codec.LiveSpec(liveDocument(live), false)

	if err != nil {
		return nil, err
	}

	plannedDocument, err := utils.YAMLDocument(plannedSpec)

	if err != nil {
		return nil, err
	}

	priorDocument, err := utils.YAMLDocument(priorSpec)

	if err != nil {
		return nil, err
	}

	liveDocument, err := utils.YAMLDocument(liveSpec)

	if err != nil {
		return nil, err
	}

	if !reflect.DeepEqual(plannedDocument["endpoint"], priorDocument["endpoint"]) || !reflect.DeepEqual(priorDocument["endpoint"], liveDocument["endpoint"]) {
		return nil, nil
	}

	return live["endpoint"], nil
}

// mergeLiveBindings sets the document bindings to the planned ones merged over the live bindings, the prior state
// telling which bindings Terraform managed.
func (entryResource *entryResource[D, S]) mergeLiveBindings(document map[string]any, prior tfsdk.State, live map[string]any) error {
	_, priorSpec, err := entryResource.codec.Spec(prior.Raw)

	if err != nil {
		return err
	}

	priorDocument, err := utils.YAMLDocument(priorSpec)

	if err != nil {
		return err
	}

	planned, _ := document["bindings"].([]any)
	previous, _ := priorDocument["bindings"].([]any)
	current, _ := live["bindings"].([]any)

	document["bindings"] = mergeBindings(planned, previous, current)

	return nil
}

// catalogName returns the exact catalog name of an entry named name, with the tenant and connector name.
func (entryResource *entryResource[D, S]) catalogName(name string) string {
	return catalog.CatalogName(entryResource.tenant(), name, entryResource.connector.ConnectorName)
}

// tenant returns the tenant of the configured provider, empty until it is configured.
func (entryResource *entryResource[D, S]) tenant() string {
	if entryResource.session == nil {
		return ""
	}

	return entryResource.session.Tenant
}
