package builder

import (
	"context"
	"errors"
	"maps"
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

// bindingsPlanner returns plan with the binding attributes the configuration omits planned, its bindings paired by pairing.
type bindingsPlanner func(plan tftypes.Value, config tftypes.Value, pairing bindingPairing) (tftypes.Value, error)

// tablesMover returns the planned bindings whose table differs from the live one, as "collection: old -> new" lines.
type tablesMover func(pairing bindingPairing) []string

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
// only when its catalog name changes, with or without tenant and connector name being the same entry, warning when
// a provider tenant change is what moves it.
func (entryResource *entryResource[D, S]) ModifyPlan(ctx context.Context, modifyPlanReq resource.ModifyPlanRequest, modifyPlanResp *resource.ModifyPlanResponse) {
	var stateName types.String
	var planName types.String
	var readLive liveReader = entryResource.newLiveReader(ctx, modifyPlanReq.State.Raw)
	var readPublication publicationReader
	var pairing bindingPairing

	if modifyPlanReq.Plan.Raw.IsNull() {
		return
	}

	planned, diagnostics := entryResource.planSecretsHash(modifyPlanReq.Plan.Raw, modifyPlanReq.Config.Raw, modifyPlanReq.State.Raw)

	modifyPlanResp.Diagnostics.Append(diagnostics...)

	planned, err := entryResource.planCatalogName(planned)

	if err == nil {
		planned, err = entryResource.planTargets(ctx, planned, modifyPlanReq.Config.Raw, modifyPlanReq.State.Raw, readLive)
	}

	if err == nil {
		planned, err = withBindingNames(planned)
	}

	if err == nil {
		planned, err = planShards(planned, modifyPlanReq.Config.Raw, modifyPlanReq.State.Raw)
	}

	if err == nil {
		pairing, err = entryResource.catalogType.resourcePath.newPairing(planned, modifyPlanReq.State.Raw, readLive)
	}

	if err == nil && entryResource.catalogType.planBindings != nil {
		planned, err = entryResource.catalogType.planBindings(planned, modifyPlanReq.Config.Raw, pairing)
	}

	if err == nil && entryResource.catalogType.planBindings != nil {
		pairing, err = pairing.withPlan(planned)
	}

	if err != nil {
		modifyPlanResp.Diagnostics.AddError("Planning "+entryResource.catalogType.name+" bindings", err.Error())
		return
	}

	modifyPlanResp.Diagnostics.Append(validateBackfillTriggers(pairing, entryResource.catalogType)...)
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

	if planName.IsUnknown() || entryResource.catalogName(planName.ValueString()) != entryResource.storedCatalogName(modifyPlanReq.State.Raw, stateName.ValueString()) {
		modifyPlanResp.RequiresReplace = append(modifyPlanResp.RequiresReplace, path.Root(nameAttribute), path.Root(catalogNameAttribute))
		modifyPlanResp.Diagnostics.Append(entryResource.warnTenantChange(modifyPlanReq.State.Raw, stateName.ValueString(), planName)...)
		return
	}

	readPublication = entryResource.newPublicationReader(pairing)

	modifyPlanResp.Diagnostics.Append(entryResource.warnTakenOverBindings(pairing)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnMovedTables(pairing)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnNamingChange(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnRemovedBindings(pairing)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnUnmanagedBindingsRemoval(pairing)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnDataflowResets(pairing, readPublication)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnEntryBackfill(modifyPlanResp.Plan.Raw, modifyPlanReq.State.Raw, readPublication)...)
	modifyPlanResp.Diagnostics.Append(entryResource.validateMissingBackfills(readPublication)...)
	modifyPlanResp.Diagnostics.Append(entryResource.warnDisabledLive(modifyPlanReq.Config.Raw, modifyPlanReq.State.Raw)...)
}

func (entryResource *entryResource[D, S]) Create(ctx context.Context, createReq resource.CreateRequest, createResp *resource.CreateResponse) {
	fingerprint, diagnostics := entryResource.apply(ctx, createReq.Plan, createReq.Config, nil, &createResp.State)

	createResp.Diagnostics.Append(diagnostics...)

	if !createResp.Diagnostics.HasError() {
		createResp.Diagnostics.Append(storeFingerprint(ctx, optionalPrivate(createResp.Private), fingerprint)...)
	}
}

func (entryResource *entryResource[D, S]) Read(ctx context.Context, readReq resource.ReadRequest, readResp *resource.ReadResponse) {
	var name types.String

	readResp.Diagnostics.Append(readReq.State.GetAttribute(ctx, path.Root(nameAttribute), &name)...)

	if readResp.Diagnostics.HasError() {
		return
	}

	stored, diagnostics := storedFingerprint(ctx, optionalPrivate(readReq.Private))

	readResp.Diagnostics.Append(diagnostics...)

	if readResp.Diagnostics.HasError() {
		return
	}

	fingerprint, diagnostics := entryResource.readState(ctx, name.ValueString(), stored, readReq, readResp)

	readResp.Diagnostics.Append(diagnostics...)

	if !readResp.Diagnostics.HasError() && !readResp.State.Raw.IsNull() {
		readResp.Diagnostics.Append(storeFingerprint(ctx, optionalPrivate(readResp.Private), fingerprint)...)
	}
}

// readState refreshes state from the live spec of its stored catalog name, removing deleted entries, and returns the
// secrets fingerprint to store, see refreshedFingerprint. An undecodable live spec fails, as the plan couldn't show it.
func (entryResource *entryResource[D, S]) readState(ctx context.Context, name string, stored string, readReq resource.ReadRequest, readResp *resource.ReadResponse) (string, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	var catalogName string = entryResource.storedCatalogName(readReq.State.Raw, name)

	sealed, err := entryResource.readSealed(ctx, catalogName)

	if err != nil {
		diagnostics.AddError("Reading "+entryResource.catalogType.name+" "+catalogName, err.Error())
		return stored, diagnostics
	}

	if sealed == nil {
		readResp.State.RemoveResource(ctx)
		return stored, diagnostics
	}

	refreshed, redactedPaths, err := entryResource.refreshedState(name, readReq.State.Raw, sealed)

	if err == nil {
		refreshed, err = withCatalogName(refreshed, catalogName)
	}

	if err != nil {
		diagnostics.AddError("Refreshing "+entryResource.catalogType.name+" "+catalogName,
			err.Error()+"\n\nThe live spec doesn't match the provider schema, so the plan can't show it. Fix it in Estuary, "+
				"then plan again.")
		return stored, diagnostics
	}

	fingerprint, refreshed, fingerprintDiagnostics := entryResource.refreshedFingerprint(catalogName, stored, sealed, refreshed)

	diagnostics.Append(fingerprintDiagnostics...)

	redactedCredentialsWarning(&diagnostics, catalogName, redactedPaths)

	readResp.State.Raw = refreshed

	return fingerprint, diagnostics
}

// refreshedState returns state refreshed from the sealed live spec, and the paths whose credentials it redacted. Live
// bindings state doesn't hold stay unmanaged, out of state, unless keep_unmanaged_bindings is false.
func (entryResource *entryResource[D, S]) refreshedState(name string, state tftypes.Value, sealed map[string]any) (tftypes.Value, []*tftypes.AttributePath, error) {
	live, redactedPaths, err := entryResource.codec.liveValue(name, sealed)

	if err != nil {
		return state, nil, err
	}

	refreshed, err := entryResource.codec.Refresh(state, live, !keepsUnmanagedBindings(state))

	if err != nil {
		return state, nil, err
	}

	return refreshed, redactedPaths, nil
}

func (entryResource *entryResource[D, S]) Update(ctx context.Context, updateReq resource.UpdateRequest, updateResp *resource.UpdateResponse) {
	fingerprint, diagnostics := entryResource.apply(ctx, updateReq.Plan, updateReq.Config, &updateReq.State, &updateResp.State)

	updateResp.Diagnostics.Append(diagnostics...)

	if !updateResp.Diagnostics.HasError() {
		updateResp.Diagnostics.Append(storeFingerprint(ctx, optionalPrivate(updateResp.Private), fingerprint)...)
	}
}

func (entryResource *entryResource[D, S]) Delete(ctx context.Context, deleteReq resource.DeleteRequest, deleteResp *resource.DeleteResponse) {
	var name types.String

	deleteResp.Diagnostics.Append(deleteReq.State.GetAttribute(ctx, path.Root(nameAttribute), &name)...)

	if deleteResp.Diagnostics.HasError() {
		return
	}

	var catalogName string = entryResource.storedCatalogName(deleteReq.State.Raw, name.ValueString())

	backend, err := entryResource.backend()

	if err == nil {
		err = backend.DeleteCatalogEntry(ctx, catalogName)
	}

	if err != nil {
		deleteResp.Diagnostics.AddError("Deleting "+entryResource.catalogType.name+" "+catalogName, err.Error())
	}
}

// ImportState imports by catalog name, stored with the tenant and without the connector name, or whole when
// stripping it would address another entry. Secrets are write-only, and bindings the control plane may add stay
// unmanaged, out of state.
func (entryResource *entryResource[D, S]) ImportState(ctx context.Context, importReq resource.ImportStateRequest, importResp *resource.ImportStateResponse) {
	var name string = catalog.Name(importReq.ID, entryResource.connector.ConnectorName)

	if entryResource.catalogName(name) != importReq.ID {
		name = importReq.ID
	}

	var catalogName string = entryResource.catalogName(name)
	var value tftypes.Value
	var redactedPaths []*tftypes.AttributePath

	sealed, err := entryResource.readSealed(ctx, catalogName)

	if err != nil {
		importResp.Diagnostics.AddError("Importing "+entryResource.catalogType.name+" "+catalogName, err.Error())
		return
	}

	if sealed == nil {
		importResp.Diagnostics.AddError("Importing "+entryResource.catalogType.name+" "+catalogName, "Estuary has no "+entryResource.catalogType.name+" named "+catalogName+". Check the name, including its tenant.")
		return
	}

	var live map[string]any = liveDocument(sealed)

	if _, err := decode[S](live, true); err != nil {
		importResp.Diagnostics.AddWarning("Importing "+entryResource.catalogType.name+" "+catalogName+" without the fields the provider doesn't manage",
			err.Error()+"\n\nThese fields aren't in the imported state. The next apply keeps them at the top level of the spec. "+
				"It keeps them in the endpoint too, unless it publishes the endpoint, which happens when the configuration sets "+
				"secrets or changes the endpoint. It removes them from the bindings declared in the configuration.")
	}

	spec, err := decode[S](live, false)

	if err == nil {
		value, redactedPaths, err = entryResource.codec.liveValue(name, sealed)
	}

	if err == nil && generatesBindings(spec) {
		var unbound map[string]any = maps.Clone(sealed)

		delete(unbound, catalog.BindingsKey)

		value, _, err = entryResource.codec.liveValue(name, unbound)
	}

	if err == nil {
		value, err = withCatalogName(value, catalogName)
	}

	if err != nil {
		importResp.Diagnostics.AddError("Importing "+entryResource.catalogType.name+" "+catalogName, err.Error())
		return
	}

	redactedCredentialsWarning(&importResp.Diagnostics, catalogName, redactedPaths)

	importResp.State.Raw = value

	importResp.Diagnostics.Append(storeFingerprint(ctx, optionalPrivate(importResp.Private), catalog.SecretsFingerprint(sealed))...)
}

// apply publishes the plan, with secrets from config, and stores it as state, returning the secrets fingerprint of the
// published entry. Updates that leave the live spec unchanged, such as a trigger strategy change, publish nothing.
func (entryResource *entryResource[D, S]) apply(ctx context.Context, plan tfsdk.Plan, config tfsdk.Config, prior *tfsdk.State, state *tfsdk.State) (string, diag.Diagnostics) {
	var diagnostics diag.Diagnostics

	planned, err := entryResource.codec.secrets.withConfigSecrets(plan.Raw, config.Raw)

	if err != nil {
		diagnostics.AddError("Converting "+entryResource.catalogType.name+" plan", err.Error())
		return "", diagnostics
	}

	if paths := entryResource.codec.secrets.credentialPaths(planned); len(paths) > 0 {
		diagnostics.AddError("Credentials in connection string",
			strings.Join(paths, ", ")+" of the "+entryResource.catalogType.name+" holds credentials, such as user:password@ or a password, token, API key or signature option, "+
				"which the catalog entry and state would store in plain text. Set them in the connector's write-only attributes instead.")
		return "", diagnostics
	}

	name, spec, err := entryResource.codec.Spec(planned)

	if err != nil {
		diagnostics.AddError("Converting "+entryResource.catalogType.name+" plan", err.Error())
		return "", diagnostics
	}

	var catalogName string = entryResource.catalogName(name)

	backend, err := entryResource.backend()

	if err != nil {
		diagnostics.AddError("Publishing "+entryResource.catalogType.name+" "+catalogName, err.Error())
		return "", diagnostics
	}

	document, err := utils.YAMLDocument(spec)

	if err != nil {
		diagnostics.AddError("Converting "+entryResource.catalogType.name+" plan", err.Error())
		return "", diagnostics
	}

	ctx = catalog.WithSecrets(ctx, entryResource.codec.secrets.values(planned)...)

	var publication publication = publication{discover: prior == nil && addsNewBindings(spec)}

	publication.identifier, _ = any(spec).(catalog.BindingsIdentifier)

	var live map[string]any
	var conflictHint string

	// A create asserts no entry has the name yet, so it never publishes over one another resource or Estuary holds.
	// An update asserts the publication ID it read, see mergeLive.
	if prior == nil {
		document[catalog.ExpectPubIDKey] = catalog.NewEntryPubID
		conflictHint = "\n\n" + catalogName + " already exists in Estuary: import it with terraform import instead of creating it."
	} else {
		publication.resets, live, err = entryResource.mergeLive(ctx, name, catalogName, document, keepsUnmanagedBindings(plan.Raw), plan.Raw, prior.Raw)

		if err != nil {
			diagnostics.AddError("Merging "+entryResource.catalogType.name+" "+catalogName+" with its live spec", err.Error())
			return "", diagnostics
		}

		conflictHint = "\n\nAnother publication changed " + catalogName + " while this apply ran, such as a dashboard edit or " +
			"an auto-discovery, so nothing was published: run terraform plan and apply again."
	}

	if live != nil && len(publication.resets) == 0 && reflect.DeepEqual(document, live) {
		tflog.Debug(ctx, "Skipping the publication, the spec is unchanged", map[string]any{"name": catalogName})
	} else if err := entryResource.catalogType.publish(ctx, backend, catalogName, document, publication); err != nil {
		var detail string = err.Error()

		if errors.Is(err, catalog.ErrPublicationConflict) {
			detail += conflictHint
		}

		diagnostics.AddError("Publishing "+entryResource.catalogType.name+" "+catalogName, detail)
		return "", diagnostics
	}

	stored, err := entryResource.codec.secrets.stateHashes(plan.Raw, planned)

	if planned := blockAttribute(stored, catalogNameAttribute, tftypes.String); err == nil && !planned.IsKnown() {
		stored, err = withCatalogName(stored, catalogName)
	}

	if err != nil {
		diagnostics.AddError("Hashing "+entryResource.catalogType.name+" "+catalogName+" secrets", err.Error())
		return "", diagnostics
	}

	fingerprint, fingerprintDiagnostics := entryResource.publishedFingerprint(ctx, catalogName, document, planned)

	diagnostics.Append(fingerprintDiagnostics...)

	state.Raw = stored

	return fingerprint, diagnostics
}
