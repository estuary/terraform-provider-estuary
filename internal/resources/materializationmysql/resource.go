package materializationmysql

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/mysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// New returns the estuary_materialization_mysql resource.
var New func() resource.Resource = builder.NewMaterialization[mysql.Config, mysql.Resource](builder.Connector{
	TypeName:      "materialization_mysql",
	Image:         mysql.ImageMySQL,
	ConnectorName: mysql.ConnectorNameMySQL,
	Description:   "Materializes Estuary collections into tables of a MySQL database, such as Google Cloud SQL or Azure Database for MySQL.",
})

// NewAurora returns the estuary_materialization_aurora_mysql resource.
var NewAurora func() resource.Resource = builder.NewMaterialization[mysql.Config, mysql.Resource](builder.Connector{
	TypeName:      "materialization_aurora_mysql",
	Image:         mysql.ImageAmazonAurora,
	ConnectorName: mysql.ConnectorNameAmazonAurora,
	Description:   "Materializes Estuary collections into tables of an Amazon Aurora for MySQL database.",
})

// NewRDS returns the estuary_materialization_rds_mysql resource.
var NewRDS func() resource.Resource = builder.NewMaterialization[mysql.Config, mysql.Resource](builder.Connector{
	TypeName:      "materialization_rds_mysql",
	Image:         mysql.ImageAmazonRDS,
	ConnectorName: mysql.ConnectorNameAmazonRDS,
	Description:   "Materializes Estuary collections into tables of an Amazon RDS for MySQL database.",
})
