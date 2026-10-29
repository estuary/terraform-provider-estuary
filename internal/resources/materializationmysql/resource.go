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
})
