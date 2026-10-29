package sourcerdsmysql

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/mysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var connector builder.Connector = builder.Connector{
	TypeName:      "source_rds_mysql",
	Image:         mysql.ImageAmazonRDS,
	ConnectorName: mysql.ConnectorNameAmazonRDS,
}

// New returns the estuary_source_rds_mysql resource.
var New func() resource.Resource = builder.NewCapture[mysql.Config, mysql.Resource](connector)

// NewDataSource returns the estuary_source_rds_mysql data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[mysql.Config, mysql.Resource](connector)
