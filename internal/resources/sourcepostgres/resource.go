package sourcepostgres

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/postgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var aurora builder.Connector = builder.Connector{
	TypeName:      "source_aurora_postgres",
	Image:         postgres.ImageAmazonAurora,
	ConnectorName: postgres.ConnectorNameAmazonAurora,
}

var rds builder.Connector = builder.Connector{
	TypeName:      "source_rds_postgres",
	Image:         postgres.ImageAmazonRDS,
	ConnectorName: postgres.ConnectorNameAmazonRDS,
}

// NewAurora returns the estuary_source_aurora_postgres resource.
var NewAurora func() resource.Resource = builder.NewCapture[postgres.Config, postgres.Resource](aurora)

// NewAuroraDataSource returns the estuary_source_aurora_postgres data source, reading the live bindings of a capture by name.
var NewAuroraDataSource func() datasource.DataSource = builder.NewCaptureDataSource[postgres.Config, postgres.Resource](aurora)

// NewRDS returns the estuary_source_rds_postgres resource.
var NewRDS func() resource.Resource = builder.NewCapture[postgres.Config, postgres.Resource](rds)

// NewRDSDataSource returns the estuary_source_rds_postgres data source, reading the live bindings of a capture by name.
var NewRDSDataSource func() datasource.DataSource = builder.NewCaptureDataSource[postgres.Config, postgres.Resource](rds)
