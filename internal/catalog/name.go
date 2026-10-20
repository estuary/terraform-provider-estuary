package catalog

import "strings"

// Tenant normalizes a tenant to end with a slash, e.g. acmeCo/.
func Tenant(tenant string) string {
	if tenant == "" || strings.HasSuffix(tenant, "/") {
		return tenant
	}

	return tenant + "/"
}

// CatalogName returns the exact catalog name of a task: name prefixed with the tenant and ending with
// the connector name, each added unless already present.
func CatalogName(tenant string, name string, connectorName string) string {
	if tenant != "" && !strings.HasPrefix(name, tenant) {
		name = tenant + name
	}

	if connectorName != "" && !strings.HasSuffix(name, "/"+connectorName) {
		name += "/" + connectorName
	}

	return name
}

// Name returns a catalog name without its connector name, keeping its tenant as an imported entry is configured.
func Name(catalogName string, connectorName string) string {
	if connectorName == "" {
		return catalogName
	}

	return strings.TrimSuffix(catalogName, "/"+connectorName)
}
