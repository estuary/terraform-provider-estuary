package builder

import (
	"context"
	"errors"
	"fmt"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// catalogType is what differs between the catalog types the provider manages: captures and materializations.
// Unset functions do nothing.
type catalogType struct {
	name             string                                                      // catalog type, as Estuary names it
	err              error                                                       // why the connector's spec types can't build this catalog type
	added            addedBindings                                               // bindings Estuary adds without Terraform
	publish          publisher                                                   // publishes the entry
	resourcePath     resourcePath                                                // identifies bindings sharing a collection
	backfillModeKey  string                                                      // binding resource key of the backfill mode, empty without one
	deltaUpdatesKey  string                                                      // binding resource key of delta updates, empty without one
	adjustAttributes func(attributes map[string]schema.Attribute)                // changes the inferred attributes
	planBindings     bindingsPlanner                                             // plans binding attributes the configuration omits
	movedTables      tablesMover                                                 // lists the bindings the plan moves to another table
	toSpecShape      func(document map[string]any)                               // moves fields from their Terraform place to their spec one
	toTerraformShape func(document map[string]any)                               // moves fields from their spec place to their Terraform one
	omittedDefault   func(path *tftypes.AttributePath, value tftypes.Value) bool // reports a non-zero default live specs omit
}

// entry is what the resource and data source of a catalog entry share.
type entry[D Document[D, S], S any] struct {
	connector   Connector
	catalogType catalogType
	codec       *Codec[D, S]
	err         error
	session     *catalog.Session
}

func newEntry[D Document[D, S], S any](connector Connector, catalogType catalogType) entry[D, S] {
	codec, err := newCodec[D, S](connector, catalogType)

	return entry[D, S]{
		connector:   connector,
		catalogType: catalogType,
		codec:       codec,
		err:         err,
	}
}

// schemaError reports whether the schema couldn't be built, adding the error to diagnostics.
func (entry *entry[D, S]) schemaError(diagnostics *diag.Diagnostics) bool {
	if entry.err != nil {
		diagnostics.AddError("Invalid connector schema", entry.err.Error())
	}

	return entry.err != nil
}

// configure stores the provider session, nil until the provider is configured.
func (entry *entry[D, S]) configure(providerData any, diagnostics *diag.Diagnostics) {
	if providerData == nil {
		return
	}

	session, ok := providerData.(*catalog.Session)

	if !ok {
		diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *catalog.Session, got %T. This is a bug in the provider: please report it.", providerData))
	}

	entry.session = session
}

// catalogName returns the full catalog name, with tenant and connector name.
func (entry *entry[D, S]) catalogName(name string) string {
	return entry.connector.catalogName(entry.session, name)
}

// backend returns the backend of the provider session, failing until the provider is configured.
func (entry *entry[D, S]) backend() (catalog.Backend, error) {
	if entry.session == nil || entry.session.Backend == nil {
		return nil, errors.New("the provider isn't configured")
	}

	return entry.session.Backend, nil
}

// readSealed reads the sealed live spec by catalog name, nil when no entry has it.
func (entry *entry[D, S]) readSealed(ctx context.Context, catalogName string) (map[string]any, error) {
	backend, err := entry.backend()

	if err != nil {
		return nil, err
	}

	return backend.ReadSpec(ctx, catalogName)
}

// readLiveValue reads the live value of the entry named name at catalogName, failing when no entry has it.
func (entry *entry[D, S]) readLiveValue(ctx context.Context, name string, catalogName string) (tftypes.Value, error) {
	sealed, err := entry.readSealed(ctx, catalogName)

	if err != nil {
		return tftypes.Value{}, err
	}

	if sealed == nil {
		return tftypes.Value{}, fmt.Errorf("Estuary has no %s named %s", entry.catalogType.name, catalogName)
	}

	live, _, err := entry.codec.liveValue(name, sealed)

	return live, err
}
