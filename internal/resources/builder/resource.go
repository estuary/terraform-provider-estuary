package builder

import (
	"context"
	"maps"
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

// entryResource is the Terraform resource of a catalog entry, a capture or a materialization.
type entryResource[D Document[D, S], S any] struct {
	entry[D, S]
}

// Compile-time checks that entryResource implements the framework resource interfaces.
var _ resource.ResourceWithConfigure = &entryResource[captureDocument[struct{}, struct{}], CaptureSpec[struct{}, struct{}]]{}
var _ resource.ResourceWithImportState = &entryResource[captureDocument[struct{}, struct{}], CaptureSpec[struct{}, struct{}]]{}
var _ resource.ResourceWithModifyPlan = &entryResource[captureDocument[struct{}, struct{}], CaptureSpec[struct{}, struct{}]]{}

func newResource[D Document[D, S], S any](connector Connector, catalogType catalogType) func() resource.Resource {
	return func() resource.Resource {
		return &entryResource[D, S]{
			entry: newEntry[D, S](connector, catalogType),
		}
	}
}

func (entryResource *entryResource[D, S]) Metadata(_ context.Context, metadataReq resource.MetadataRequest, metadataResp *resource.MetadataResponse) {
	metadataResp.TypeName = entryResource.connector.typeName(metadataReq.ProviderTypeName)
}

func (entryResource *entryResource[D, S]) Schema(_ context.Context, _ resource.SchemaRequest, schemaResp *resource.SchemaResponse) {
	if !entryResource.schemaError(&schemaResp.Diagnostics) {
		schemaResp.Schema = entryResource.codec.Schema()
	}
}

func (entryResource *entryResource[D, S]) Configure(_ context.Context, configureReq resource.ConfigureRequest, configureResp *resource.ConfigureResponse) {
	entryResource.configure(configureReq.ProviderData, &configureResp.Diagnostics)
}

// ModifyPlan plans secret hashes, binding targets and names, and validates backfill triggers. It replaces the entry
// only when its catalog name changes, with or without tenant and connector name being the same entry.
func (entryResource *entryResource[D, S]) ModifyPlan(ctx context.Context, modifyPlanReq resource.ModifyPlanRequest, modifyPlanResp *resource.ModifyPlanResponse) {
	var stateName types.String
	var planName types.String
	var readLive liveReader = entryResource.newLiveReader(ctx, modifyPlanReq.State.Raw)
	var readPublication publicationReader

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

	if err == nil {
		planned, err = planBindingFields(planned, modifyPlanReq.Config.Raw, modifyPlanReq.State.Raw, readLive)
	}

	if err == nil {
		planned, err = entryResource.codec.planLiveBindings(planned, modifyPlanReq.State.Raw)
	}

	if err != nil {
		modifyPlanResp.Diagnostics.AddError("Planning "+entryResource.catalogType.name+" bindings", err.Error())
	}

	modifyPlanResp.Diagnostics.Append(validateBackfillTriggers(planned, modifyPlanReq.State.Raw, entryResource.catalogType)...)
	modifyPlanResp.Diagnostics.Append(entryResource.validateKeepUnmanagedBindings(planned)...)

	modifyPlanResp.Plan.Raw = planned

	if !modifyPlanResp.Diagnostics.HasError() {
		modifyPlanResp.Diagnostics.Append(entryResource.warnAddedBindings(planned, modifyPlanReq.State.Raw)...)
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

	readPublication = entryResource.newPublicationReader(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw, readLive)

	modifyPlanResp.Diagnostics.Append(entryResource.warnTakenOverBindings(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw, readLive)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnRemovedBindings(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnUnmanagedBindingsRemoval(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw, readLive)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnDataflowResets(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw, readPublication)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnEntryBackfill(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw, readPublication)...)
	modifyPlanResp.Diagnostics.Append(entryResource.validateModeBackfills(modifyPlanResp.Plan.Raw, readPublication)...)
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

	readResp.Diagnostics.Append(entryResource.readState(ctx, name.ValueString(), readReq, readResp)...)
}

// readState refreshes state from the live spec, removing deleted entries. An undecodable live spec keeps state with
// a warning, rather than failing every plan.
func (entryResource *entryResource[D, S]) readState(ctx context.Context, name string, readReq resource.ReadRequest, readResp *resource.ReadResponse) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var catalogName string = entryResource.catalogName(name)

	sealed, err := entryResource.readSealed(ctx, catalogName)

	if err != nil {
		diagnostics.AddError("Reading "+entryResource.catalogType.name+" "+catalogName, err.Error())
		return diagnostics
	}

	if sealed == nil {
		readResp.State.RemoveResource(ctx)
		return diagnostics
	}

	refreshed, redactedPaths, err := entryResource.refreshedState(name, readReq.State.Raw, sealed)

	if err != nil {
		diagnostics.AddWarning("Couldn't refresh "+entryResource.catalogType.name+" "+catalogName,
			err.Error()+"\n\nThe plan doesn't show changes made outside Terraform.")
		return diagnostics
	}

	redactedCredentialsWarning(&diagnostics, catalogName, redactedPaths)

	readResp.State.Raw = refreshed

	return diagnostics
}

// refreshedState returns state refreshed from the sealed live spec, and the paths whose credentials it redacted. Live
// bindings state doesn't hold stay unmanaged unless keep_unmanaged_bindings is false, and live_bindings, when the
// entry has it, lists them all.
func (entryResource *entryResource[D, S]) refreshedState(name string, state tftypes.Value, sealed map[string]any) (tftypes.Value, []*tftypes.AttributePath, error) {
	live, redactedPaths, err := entryResource.codec.liveValue(name, sealed)

	if err != nil {
		return state, nil, err
	}

	refreshed, err := entryResource.codec.Refresh(state, live, !keepsUnmanagedBindings(state))

	if err != nil {
		return state, nil, err
	}

	refreshed, err = withLiveBindings(refreshed, live)

	return refreshed, redactedPaths, err
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

	backend, err := entryResource.backend()

	if err == nil {
		err = backend.DeleteCatalogEntry(ctx, catalogName)
	}

	if err != nil {
		deleteResp.Diagnostics.AddError("Deleting "+entryResource.catalogType.name+" "+catalogName, err.Error())
	}
}

// ImportState imports by catalog name, stored with the tenant and without the connector name. Secrets are
// write-only, and bindings the control plane may add stay unmanaged, listed only in live_bindings when it exists.
func (entryResource *entryResource[D, S]) ImportState(ctx context.Context, importReq resource.ImportStateRequest, importResp *resource.ImportStateResponse) {
	var name string = catalog.Name(importReq.ID, entryResource.connector.ConnectorName)
	var catalogName string = entryResource.catalogName(name)
	var liveEntry tftypes.Value
	var redactedPaths []*tftypes.AttributePath

	sealed, err := entryResource.readSealed(ctx, catalogName)

	if err != nil {
		importResp.Diagnostics.AddError("Importing "+entryResource.catalogType.name+" "+catalogName, err.Error())
		return
	}

	if sealed == nil {
		importResp.Diagnostics.AddError("Importing "+entryResource.catalogType.name+" "+catalogName, "No catalog entry is named "+catalogName+".")
		return
	}

	var live map[string]any = liveDocument(sealed)

	if _, err := decode[S](live, true); err != nil {
		importResp.Diagnostics.AddWarning("Importing "+entryResource.catalogType.name+" "+catalogName+" without the fields the provider doesn't manage",
			err.Error()+"\n\nThese fields aren't in the imported state. The next apply keeps the unknown top-level fields, and "+
				"the unknown endpoint fields until it publishes the endpoint, which it does when the configuration sets secrets "+
				"or changes the endpoint. It removes the unknown fields of the bindings declared in the configuration.")
	}

	spec, err := decode[S](live, false)

	if err == nil {
		liveEntry, redactedPaths, err = entryResource.codec.liveValue(name, sealed)
	}

	var value tftypes.Value = liveEntry

	if err == nil && generatesBindings(spec) {
		var unbound map[string]any = maps.Clone(sealed)

		delete(unbound, catalog.BindingsKey)

		value, _, err = entryResource.codec.liveValue(name, unbound)
	}

	if err == nil {
		value, err = withLiveBindings(value, liveEntry)
	}

	if err != nil {
		importResp.Diagnostics.AddError("Importing "+entryResource.catalogType.name+" "+catalogName, err.Error())
		return
	}

	redactedCredentialsWarning(&importResp.Diagnostics, catalogName, redactedPaths)

	importResp.State.Raw = value
}

// apply publishes the plan, with secrets from config, and stores it as state. Updates that leave the live spec
// unchanged, such as a trigger strategy change, publish nothing.
func (entryResource *entryResource[D, S]) apply(ctx context.Context, plan tfsdk.Plan, config tfsdk.Config, prior *tfsdk.State, state *tfsdk.State) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	planned, err := entryResource.codec.secrets.withConfigSecrets(plan.Raw, config.Raw)

	if err != nil {
		diagnostics.AddError("Converting "+entryResource.catalogType.name+" plan", err.Error())
		return diagnostics
	}

	name, spec, err := entryResource.codec.Spec(planned)

	if err != nil {
		diagnostics.AddError("Converting "+entryResource.catalogType.name+" plan", err.Error())
		return diagnostics
	}

	var catalogName string = entryResource.catalogName(name)

	backend, err := entryResource.backend()

	if err != nil {
		diagnostics.AddError("Publishing "+entryResource.catalogType.name+" "+catalogName, err.Error())
		return diagnostics
	}

	document, err := utils.YAMLDocument(spec)

	if err != nil {
		diagnostics.AddError("Converting "+entryResource.catalogType.name+" plan", err.Error())
		return diagnostics
	}

	ctx = catalog.WithSecrets(ctx, entryResource.codec.secrets.values(planned)...)

	var publication publication = publication{discover: prior == nil && addsNewBindings(spec)}

	publication.identifier, _ = any(spec).(catalog.BindingsIdentifier)

	var live map[string]any

	if prior != nil {
		publication.resets, live, err = entryResource.mergeLive(ctx, catalogName, document, keepsUnmanagedBindings(plan.Raw), plan.Raw, prior.Raw)

		if err != nil {
			diagnostics.AddError("Merging "+entryResource.catalogType.name+" "+catalogName+" with its live spec", err.Error())
			return diagnostics
		}
	}

	if live != nil && len(publication.resets) == 0 && reflect.DeepEqual(document, live) {
		tflog.Debug(ctx, "Skipping the publication, the spec is unchanged", map[string]any{"name": catalogName})
	} else if err := entryResource.catalogType.publish(ctx, backend, catalogName, document, publication); err != nil {
		diagnostics.AddError("Publishing "+entryResource.catalogType.name+" "+catalogName, err.Error())
		return diagnostics
	}

	stored, err := entryResource.codec.secrets.stateHashes(plan.Raw, planned)

	if err != nil {
		diagnostics.AddError("Hashing "+entryResource.catalogType.name+" "+catalogName+" secrets", err.Error())
		return diagnostics
	}

	stored, liveDiagnostics := entryResource.withPublishedLiveBindings(ctx, name, catalogName, stored)

	diagnostics.Append(liveDiagnostics...)

	state.Raw = stored

	return diagnostics
}
