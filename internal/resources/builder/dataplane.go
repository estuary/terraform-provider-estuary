package builder

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// planDataPlane plans data_plane: for a new entry, a replacing one included, the configured one, else the provider
// one, else unknown, and for a live entry the state one, failing on another configured one, as Estuary can't move it.
func (entryResource *entryResource[D, S]) planDataPlane(plan tftypes.Value, config tftypes.Value, state tftypes.Value) (tftypes.Value, diag.Diagnostics, error) {
	var diagnostics diag.Diagnostics
	var configured tftypes.Value = blockAttribute(config, dataPlaneAttribute, tftypes.String)
	var dataPlane tftypes.Value = configured

	live, isLive := known[string](blockAttribute(state, dataPlaneAttribute, tftypes.String))
	wanted, isConfigured := known[string](configured)

	switch {
	case !configured.IsKnown():
	case !exists(state) || entryResource.replacesEntry(plan, state):
		if !isConfigured && entryResource.session != nil && entryResource.session.DataPlane != "" {
			dataPlane = tftypes.NewValue(tftypes.String, entryResource.session.DataPlane)
		} else if !isConfigured {
			dataPlane = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		}
	case !isConfigured:
		dataPlane = blockAttribute(state, dataPlaneAttribute, tftypes.String)
	case isLive && wanted != live:
		diagnostics.AddAttributeError(path.Root(dataPlaneAttribute), "Data plane of a live "+entryResource.catalogType.name+" can't change",
			"This "+entryResource.catalogType.name+" runs in the data plane "+live+", but data_plane is set to "+wanted+". "+
				"Estuary can't move a live "+entryResource.catalogType.name+" to another data plane, so nothing is published or "+
				"replaced.\n\nTo keep it where it is, set data_plane to "+live+" or remove the attribute. To move it, ask "+
				"Estuary support to migrate it to "+wanted+", which keeps its state, and once it's migrated set data_plane to "+
				"the data plane it then runs in.")
	}

	planned, err := withAttribute(plan, dataPlanePath, func(tftypes.Value) tftypes.Value {
		return dataPlane
	})

	return planned, diagnostics, err
}

// replacesEntry reports whether the planned catalog name, see planCatalogName, is unknown or another than the one state
// addresses, which replaces the entry.
func (entryResource *entryResource[D, S]) replacesEntry(plan tftypes.Value, state tftypes.Value) bool {
	name, _ := known[string](blockAttribute(state, nameAttribute, tftypes.String))
	planned, isKnown := known[string](blockAttribute(plan, catalogNameAttribute, tftypes.String))

	return !isKnown || planned != entryResource.storedCatalogName(state, name)
}

// plannedDataPlane returns the data_plane of value, empty while null or unknown, which leaves the choice to Estuary.
func plannedDataPlane(value tftypes.Value) string {
	dataPlane, _ := known[string](blockAttribute(value, dataPlaneAttribute, tftypes.String))

	return dataPlane
}

// readDataPlane returns the name of the data plane the live entry runs in, reporting false, with a logged warning, when
// it can't be read, which never fails the caller: tokens may not read the name of every data plane.
func (entryResource *entryResource[D, S]) readDataPlane(ctx context.Context, catalogName string) (string, bool) {
	var dataPlane string

	backend, err := entryResource.backend()

	if err == nil {
		dataPlane, err = backend.ReadDataPlane(ctx, catalogName)
	}

	if err != nil {
		tflog.Warn(ctx, "Reading the data plane of "+catalogName, map[string]any{"error": err.Error()})
		return "", false
	}

	if dataPlane == "" {
		tflog.Warn(ctx, "Estuary didn't return the name of the data plane of "+catalogName)
	}

	return dataPlane, dataPlane != ""
}

// withLiveDataPlane returns value with data_plane set to the data plane the live entry runs in, unchanged when it
// can't be read, see readDataPlane, so state keeps the one it held.
func (entryResource *entryResource[D, S]) withLiveDataPlane(ctx context.Context, value tftypes.Value, catalogName string) (tftypes.Value, error) {
	dataPlane, found := entryResource.readDataPlane(ctx, catalogName)

	if !found {
		return value, nil
	}

	return withAttribute(value, dataPlanePath, func(tftypes.Value) tftypes.Value {
		return tftypes.NewValue(tftypes.String, dataPlane)
	})
}

// publishedDataPlane returns stored with a data_plane the plan couldn't know, see planDataPlane, set to the one Estuary
// created the published entry in, null when it can't be read, for the next refresh to fill in.
func (entryResource *entryResource[D, S]) publishedDataPlane(ctx context.Context, catalogName string, stored tftypes.Value) (tftypes.Value, error) {
	if blockAttribute(stored, dataPlaneAttribute, tftypes.String).IsKnown() {
		return stored, nil
	}

	return withAttribute(stored, dataPlanePath, func(tftypes.Value) tftypes.Value {
		if dataPlane, found := entryResource.readDataPlane(ctx, catalogName); found {
			return tftypes.NewValue(tftypes.String, dataPlane)
		}

		return tftypes.NewValue(tftypes.String, nil)
	})
}
