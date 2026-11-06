package materializationiceberg

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/iceberg"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// New returns the estuary_materialization_iceberg resource.
var New func() resource.Resource = builder.NewMaterialization[iceberg.Config, iceberg.Resource](builder.Connector{
	TypeName:      "materialization_iceberg",
	Image:         iceberg.Image,
	ConnectorName: iceberg.ConnectorName,
	Description:   "Materializes Estuary collections into Apache Iceberg tables of a REST catalog, merging changes into one row per key with Spark jobs on AWS EMR Serverless.",
})
