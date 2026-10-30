package materializations3iceberg

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/icebergdelta"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// New returns the estuary_materialization_s3_iceberg resource.
var New func() resource.Resource = builder.NewMaterialization[icebergdelta.Config, icebergdelta.Resource](builder.Connector{
	TypeName:      "materialization_s3_iceberg",
	Image:         icebergdelta.Image,
	ConnectorName: icebergdelta.ConnectorName,
})
