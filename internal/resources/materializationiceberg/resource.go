package materializationiceberg

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/icebergs3"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// New returns the estuary_materialization_iceberg resource.
var New func() resource.Resource = builder.NewMaterialization[icebergs3.Config, icebergs3.Resource](builder.Connector{
	TypeName:      "materialization_iceberg",
	Image:         icebergs3.Image,
	ConnectorName: icebergs3.ConnectorName,
	Description:   "Materializes Estuary collections into Apache Iceberg tables of a REST catalog, merging changes into one row per key with Spark jobs on AWS EMR Serverless.",
})
