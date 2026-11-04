package sourcepostgres

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/postgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var self builder.Connector = builder.Connector{
	TypeName:      "source_postgres",
	Image:         postgres.ImagePostgres,
	ConnectorName: postgres.ConnectorNamePostgres,
	Description:   "Captures tables of a PostgreSQL database, such as a self-hosted server, Google Cloud SQL or Azure Database for PostgreSQL, into Estuary collections, through change data capture with PostgreSQL logical replication.",
}

var aurora builder.Connector = builder.Connector{
	TypeName:      "source_aurora_postgres",
	Image:         postgres.ImageAmazonAurora,
	ConnectorName: postgres.ConnectorNameAmazonAurora,
	Description:   "Captures tables of an Amazon Aurora for PostgreSQL database into Estuary collections, through change data capture with PostgreSQL logical replication.",
}

var rds builder.Connector = builder.Connector{
	TypeName:      "source_rds_postgres",
	Image:         postgres.ImageAmazonRDS,
	ConnectorName: postgres.ConnectorNameAmazonRDS,
	Description:   "Captures tables of an Amazon RDS for PostgreSQL database into Estuary collections, through change data capture with PostgreSQL logical replication.",
}

// New returns the estuary_source_postgres resource.
var New func() resource.Resource = builder.NewCapture[postgres.Config, postgres.Resource](self)

// NewDataSource returns the estuary_source_postgres data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[postgres.Config, postgres.Resource](self)

// NewAurora returns the estuary_source_aurora_postgres resource.
var NewAurora func() resource.Resource = builder.NewCapture[postgres.Config, postgres.Resource](aurora)

// NewAuroraDataSource returns the estuary_source_aurora_postgres data source, reading the live bindings of a capture by name.
var NewAuroraDataSource func() datasource.DataSource = builder.NewCaptureDataSource[postgres.Config, postgres.Resource](aurora)

// NewRDS returns the estuary_source_rds_postgres resource.
var NewRDS func() resource.Resource = builder.NewCapture[postgres.Config, postgres.Resource](rds)

// NewRDSDataSource returns the estuary_source_rds_postgres data source, reading the live bindings of a capture by name.
var NewRDSDataSource func() datasource.DataSource = builder.NewCaptureDataSource[postgres.Config, postgres.Resource](rds)
