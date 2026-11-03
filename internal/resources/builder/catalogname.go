package builder

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// storedCatalogName returns the catalog name state addresses: the one stored when created or imported, else, for
// state written before catalog_name existed, name under the current tenant.
func (entryResource *entryResource[D, S]) storedCatalogName(state tftypes.Value, name string) string {
	if stored, isKnown := known[string](blockAttribute(state, catalogNameAttribute, tftypes.String)); isKnown {
		return stored
	}

	return entryResource.catalogName(name)
}

// planCatalogName plans the catalog name of the planned name under the current tenant, unknown while name is.
func (entryResource *entryResource[D, S]) planCatalogName(plan tftypes.Value) (tftypes.Value, error) {
	name, isKnown := known[string](blockAttribute(plan, nameAttribute, tftypes.String))

	if !isKnown {
		return withAttribute(plan, catalogNamePath, func(tftypes.Value) tftypes.Value {
			return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		})
	}

	return withCatalogName(plan, entryResource.catalogName(name))
}

// withCatalogName returns value with catalog_name set to catalogName.
func withCatalogName(value tftypes.Value, catalogName string) (tftypes.Value, error) {
	return withAttribute(value, catalogNamePath, func(tftypes.Value) tftypes.Value {
		return tftypes.NewValue(tftypes.String, catalogName)
	})
}

// warnTenantChange warns when a provider tenant change, not name, moves the entry to another catalog name, as the
// replacement deletes the entry under the old tenant and creates it under the new one.
func (entryResource *entryResource[D, S]) warnTenantChange(state tftypes.Value, stateName string, planName types.String) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	var stored string = entryResource.storedCatalogName(state, stateName)
	var moved string = entryResource.catalogName(stateName)

	if planName.IsUnknown() || moved == stored {
		return diagnostics
	}

	diagnostics.AddAttributeWarning(path.Root(catalogNameAttribute), "Provider tenant changed: "+stored+" is replaced by "+entryResource.catalogName(planName.ValueString()),
		"The provider tenant changed, so this "+entryResource.catalogType.name+" moves to the new tenant: the apply deletes "+stored+
			" and creates it again as "+entryResource.catalogName(planName.ValueString())+", a new entry. Every resource of this "+
			"provider configuration is replaced the same way.\n\nThe Estuary token must be able to delete under the old tenant and "+
			"publish under the new one. To keep managing the old tenant, restore its tenant in the provider configuration, and "+
			"manage the new tenant through another provider configuration, using an alias.")

	return diagnostics
}
