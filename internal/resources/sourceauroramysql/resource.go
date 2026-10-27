package sourceauroramysql

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/mysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
)

var New = builder.NewCapture[mysql.Config, mysql.Resource](builder.Connector{
	TypeName:      "source_aurora_mysql",
	Image:         mysql.ImageAmazonAurora,
	ConnectorName: mysql.ConnectorNameAmazonAurora,
})
