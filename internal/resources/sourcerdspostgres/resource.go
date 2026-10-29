package sourcerdspostgres

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/postgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var connector builder.Connector = builder.Connector{
	TypeName:      "source_rds_postgres",
	Image:         postgres.ImageAmazonRDS,
	ConnectorName: postgres.ConnectorNameAmazonRDS,
}

// New returns the estuary_source_rds_postgres resource.
var New func() resource.Resource = builder.NewCapture[postgres.Config, postgres.Resource](connector)

// NewDataSource returns the estuary_source_rds_postgres data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[postgres.Config, postgres.Resource](connector)
