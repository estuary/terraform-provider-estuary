package builder

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// planDataPlane plans data_plane. An entry not created yet, or the new one a catalog name change replaces it with, is
// created in the configured data plane, else in the provider one, else in the one Estuary picks, known after the
// apply. A live entry keeps its data plane, the one state holds: the provider one never applies to it, and configuring
// another one fails, as Estuary can't move it. While state holds none, the configured one is accepted.
func (entryResource *entryResource[D, S]) planDataPlane(plan tftypes.Value, config tftypes.Value, state tftypes.Value) (tftypes.Value, diag.Diagnostics, error) {
	var diagnostics diag.Diagnostics
	var configured tftypes.Value = blockAttribute(config, dataPlaneAttribute, tftypes.String)
	var planned tftypes.Value = configured

	live, isLive := known[string](blockAttribute(state, dataPlaneAttribute, tftypes.String))
	wanted, isConfigured := known[string](configured)

	switch {
	case !configured.IsKnown():
	case !exists(state) || entryResource.replacesEntry(plan, state):
		if !isConfigured && entryResource.session != nil && entryResource.session.DataPlane != "" {
			planned = tftypes.NewValue(tftypes.String, entryResource.session.DataPlane)
		} else if !isConfigured {
			planned = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		}
	case !isConfigured:
		planned = blockAttribute(state, dataPlaneAttribute, tftypes.String)
	case isLive && wanted != live:
		diagnostics.AddAttributeError(path.Root(dataPlaneAttribute), "Data plane of a live "+entryResource.catalogType.name+" can't change",
			"This "+entryResource.catalogType.name+" runs in the data plane "+live+", but data_plane is set to "+wanted+". "+
				"Estuary can't move a live "+entryResource.catalogType.name+" to another data plane, so nothing is published or "+
				"replaced.\n\nTo keep it where it is, set data_plane to "+live+" or remove the attribute. To move it, ask "+
				"Estuary support to migrate it to "+wanted+", which keeps its state, and once it's migrated set data_plane to "+
				"the data plane it then runs in.")
	}

	planned, err := withAttribute(plan, dataPlanePath, func(tftypes.Value) tftypes.Value {
		return planned
	})

	return planned, diagnostics, err
}

// replacesEntry reports whether plan moves the entry in state to another catalog name, unknown ones included, which
// replaces it with a new entry, see ModifyPlan. Plan holds its planned catalog name, see planCatalogName.
func (entryResource *entryResource[D, S]) replacesEntry(plan tftypes.Value, state tftypes.Value) bool {
	name, _ := known[string](blockAttribute(state, nameAttribute, tftypes.String))
	planned, isKnown := known[string](blockAttribute(plan, catalogNameAttribute, tftypes.String))

	return !isKnown || planned != entryResource.storedCatalogName(state, name)
}

// plannedDataPlane returns the data plane planDataPlane planned, which publications, discoveries and dry runs name to
// Estuary, empty while it's null or unknown, leaving the choice to Estuary: the one of the live entry, else the first
// of the storage mapping.
func plannedDataPlane(plan tftypes.Value) string {
	dataPlane, _ := known[string](blockAttribute(plan, dataPlaneAttribute, tftypes.String))

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
