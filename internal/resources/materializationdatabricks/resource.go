package materializationdatabricks

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/databricks"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// New returns the estuary_materialization_databricks resource.
var New func() resource.Resource = builder.NewMaterialization[databricks.Config, databricks.Resource](builder.Connector{
	TypeName:      "materialization_databricks",
	Image:         databricks.Image,
	ConnectorName: databricks.ConnectorName,
	Description:   "Materializes Estuary collections into Databricks tables of a Unity Catalog, through a SQL warehouse.",
})
