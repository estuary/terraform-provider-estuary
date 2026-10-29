package materializationrdsmysql

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/mysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// New returns the estuary_materialization_rds_mysql resource.
var New func() resource.Resource = builder.NewMaterialization[mysql.Config, mysql.Resource](builder.Connector{
	TypeName:      "materialization_rds_mysql",
	Image:         mysql.ImageAmazonRDS,
	ConnectorName: mysql.ConnectorNameAmazonRDS,
})
