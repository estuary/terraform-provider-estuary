package catalog

import "strings"

// Tenant normalizes a tenant to end with a slash, e.g. acmeCo/.
func Tenant(tenant string) string {
	if tenant == "" || strings.HasSuffix(tenant, "/") {
		return tenant
	}

	return tenant + "/"
}

// CatalogName returns name with the tenant prefix and connector name suffix, adding each only when missing.
func CatalogName(tenant string, name string, connectorName string) string {
	if tenant != "" && !strings.HasPrefix(name, tenant) {
		name = tenant + name
	}

	if connectorName != "" && !strings.HasSuffix(name, "/"+connectorName) {
		name += "/" + connectorName
	}

	return name
}

// Name strips the connector name from a catalog name, keeping the tenant as configs write it.
func Name(catalogName string, connectorName string) string {
	if connectorName == "" {
		return catalogName
	}

	return strings.TrimSuffix(catalogName, "/"+connectorName)
}
