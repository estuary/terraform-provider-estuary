package catalog_test

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/dekaf"
)

func TestTenant(test *testing.T) {
	for tenant, want := range map[string]string{
		"acmeCo":  "acmeCo/",
		"acmeCo/": "acmeCo/",
		"":        "",
	} {
		if got := catalog.Tenant(tenant); got != want {
			test.Errorf("Tenant(%q) = %q, want %q", tenant, got, want)
		}
	}
}

func TestCatalogName(test *testing.T) {
	expectations := map[string]struct {
		tenant        string
		name          string
		connectorName string
		want          string
	}{
		"adds tenant and connector name": {"acmeCo/", "movies", "source-mongodb", "acmeCo/movies/source-mongodb"},
		"keeps a starting tenant":        {"acmeCo/", "acmeCo/movies", "source-mongodb", "acmeCo/movies/source-mongodb"},
		"keeps an ending connector name": {"acmeCo/", "movies/source-mongodb", "source-mongodb", "acmeCo/movies/source-mongodb"},
		"keeps an exact catalog name":    {"acmeCo/", "acmeCo/movies/source-mongodb", "source-mongodb", "acmeCo/movies/source-mongodb"},
		"appends to a partial match":     {"acmeCo/", "my-source-mongodb", "source-mongodb", "acmeCo/my-source-mongodb/source-mongodb"},
		"prefixes a partial tenant":      {"acmeCo/", "acmeCorp/movies", "source-mongodb", "acmeCo/acmeCorp/movies/source-mongodb"},
		"adds nothing when both empty":   {"", "acmeCo/movies", "", "acmeCo/movies"},
		"appends a dekaf variant name":   {"acmeCo/", "topics", dekaf.ConnectorName(dekaf.VariantTinybird), "acmeCo/topics/dekaf-tinybird"},
	}

	for name, expected := range expectations {
		if got := catalog.CatalogName(expected.tenant, expected.name, expected.connectorName); got != expected.want {
			test.Errorf("%s: CatalogName(%q, %q, %q) = %q, want %q", name, expected.tenant, expected.name, expected.connectorName, got, expected.want)
		}
	}
}

func TestName(test *testing.T) {
	expectations := map[string]struct {
		catalogName   string
		connectorName string
		want          string
	}{
		"dashboard name":          {"acmeCo/movies/source-mongodb", "source-mongodb", "acmeCo/movies"},
		"custom name":             {"acmeCo/movies/custom", "source-mongodb", "acmeCo/movies/custom"},
		"other tenant":            {"otherCo/movies/source-mongodb", "source-mongodb", "otherCo/movies"},
		"without connector name":  {"acmeCo/topics/", "", "acmeCo/topics/"},
		"round trips CatalogName": {catalog.CatalogName("acmeCo/", "acmeCo/movies", "source-mongodb"), "source-mongodb", "acmeCo/movies"},
	}

	for name, expected := range expectations {
		if got := catalog.Name(expected.catalogName, expected.connectorName); got != expected.want {
			test.Errorf("%s: Name(%q, %q) = %q, want %q", name, expected.catalogName, expected.connectorName, got, expected.want)
		}
	}
}
