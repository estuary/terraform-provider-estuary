package sourcemysql

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/mysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var self builder.Connector = builder.Connector{
	TypeName:      "source_mysql",
	Image:         mysql.ImageMySQL,
	ConnectorName: mysql.ConnectorNameMySQL,
	Description:   "Captures tables of a MySQL database, such as a self-hosted server, Google Cloud SQL or Azure Database for MySQL, into Estuary collections, through change data capture on the MySQL binary log.",
}

var aurora builder.Connector = builder.Connector{
	TypeName:      "source_aurora_mysql",
	Image:         mysql.ImageAmazonAurora,
	ConnectorName: mysql.ConnectorNameAmazonAurora,
	Description:   "Captures tables of an Amazon Aurora for MySQL database into Estuary collections, through change data capture on the MySQL binary log.",
}

var rds builder.Connector = builder.Connector{
	TypeName:      "source_rds_mysql",
	Image:         mysql.ImageAmazonRDS,
	ConnectorName: mysql.ConnectorNameAmazonRDS,
	Description:   "Captures tables of an Amazon RDS for MySQL database into Estuary collections, through change data capture on the MySQL binary log.",
}

// New returns the estuary_source_mysql resource.
var New func() resource.Resource = builder.NewCapture[mysql.Config, mysql.Resource](self)

// NewDataSource returns the estuary_source_mysql data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[mysql.Config, mysql.Resource](self)

// NewAurora returns the estuary_source_aurora_mysql resource.
var NewAurora func() resource.Resource = builder.NewCapture[mysql.Config, mysql.Resource](aurora)

// NewAuroraDataSource returns the estuary_source_aurora_mysql data source, reading the live bindings of a capture by name.
var NewAuroraDataSource func() datasource.DataSource = builder.NewCaptureDataSource[mysql.Config, mysql.Resource](aurora)

// NewRDS returns the estuary_source_rds_mysql resource.
var NewRDS func() resource.Resource = builder.NewCapture[mysql.Config, mysql.Resource](rds)

// NewRDSDataSource returns the estuary_source_rds_mysql data source, reading the live bindings of a capture by name.
var NewRDSDataSource func() datasource.DataSource = builder.NewCaptureDataSource[mysql.Config, mysql.Resource](rds)
