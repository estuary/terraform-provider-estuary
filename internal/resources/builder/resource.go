package builder

import (
	"context"
	"fmt"
	"reflect"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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

	modifyPlanResp.Diagnostics.Append(modifyPlanReq.State.GetAttribute(ctx, path.Root(nameAttribute), &stateName)...)
	modifyPlanResp.Diagnostics.Append(modifyPlanReq.Plan.GetAttribute(ctx, path.Root(nameAttribute), &planName)...)

	if modifyPlanResp.Diagnostics.HasError() {
		return
	}

	if planName.IsUnknown() || entryResource.catalogName(planName.ValueString()) != entryResource.catalogName(stateName.ValueString()) {
		modifyPlanResp.RequiresReplace = append(modifyPlanResp.RequiresReplace, path.Root(nameAttribute))
		return
	}

	modifyPlanResp.Diagnostics.Append(entryResource.warnTakenOverBindings(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw, readLive)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnRemovedBindings(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnDataflowResets(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw, readLive)...)
	modifyPlanResp.Diagnostics.Append(entryResource.validateModeBackfills(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw, readLive)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnDisabledLive(modifyPlanReq.Config.Raw, modifyPlanReq.State.Raw)...)
}

func (entryResource *entryResource[D, S]) Create(ctx context.Context, createReq resource.CreateRequest, createResp *resource.CreateResponse) {
	createResp.Diagnostics.Append(entryResource.apply(ctx, createReq.Plan, createReq.Config, nil, &createResp.State)...)
}

func (entryResource *entryResource[D, S]) Read(ctx context.Context, readReq resource.ReadRequest, readResp *resource.ReadResponse) {
	var name types.String

	readResp.Diagnostics.Append(readReq.State.GetAttribute(ctx, path.Root(nameAttribute), &name)...)

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

	deleteResp.Diagnostics.Append(deleteReq.State.GetAttribute(ctx, path.Root(nameAttribute), &name)...)

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
		delete(live, bindingsAttribute)

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
