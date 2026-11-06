package materializations3iceberg

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/s3iceberg"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// New returns the estuary_materialization_s3_iceberg resource.
var New func() resource.Resource = builder.NewMaterialization[s3iceberg.Config, s3iceberg.Resource](builder.Connector{
	TypeName:      "materialization_s3_iceberg",
	Image:         s3iceberg.Image,
	ConnectorName: s3iceberg.ConnectorName,
	Description:   "Materializes Estuary collections into Apache Iceberg tables in Amazon S3, appending every change as delta updates.",
})
