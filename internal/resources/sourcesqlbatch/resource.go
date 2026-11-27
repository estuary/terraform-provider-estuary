package sourcesqlbatch

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/mysqlbatch"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/redshiftbatch"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var mysql builder.Connector = builder.Connector{
	TypeName:      "source_mysql_batch",
	Image:         mysqlbatch.Image,
	ConnectorName: mysqlbatch.ConnectorName,
	Description:   "Captures MySQL tables, views and custom queries into Estuary collections, by polling them on a schedule.",
}

var redshift builder.Connector = builder.Connector{
	TypeName:      "source_redshift_batch",
	Image:         redshiftbatch.Image,
	ConnectorName: redshiftbatch.ConnectorName,
	Description:   "Captures Amazon Redshift tables, views and custom queries into Estuary collections, by polling them on a schedule.",
}

// NewMySQL returns the estuary_source_mysql_batch resource.
var NewMySQL func() resource.Resource = builder.NewCapture[mysqlbatch.Config, mysqlbatch.Resource](mysql)

// NewMySQLDataSource returns the estuary_source_mysql_batch data source, reading the live bindings of a capture by name.
var NewMySQLDataSource func() datasource.DataSource = builder.NewCaptureDataSource[mysqlbatch.Config, mysqlbatch.Resource](mysql)

// NewRedshift returns the estuary_source_redshift_batch resource.
var NewRedshift func() resource.Resource = builder.NewCapture[redshiftbatch.Config, redshiftbatch.Resource](redshift)

// NewRedshiftDataSource returns the estuary_source_redshift_batch data source, reading the live bindings of a capture by name.
var NewRedshiftDataSource func() datasource.DataSource = builder.NewCaptureDataSource[redshiftbatch.Config, redshiftbatch.Resource](redshift)
