package sourcerdspostgres

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/postgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
)

var New = builder.NewCapture[postgres.Config, postgres.Resource](builder.Connector{
	TypeName:      "source_rds_postgres",
	Image:         postgres.ImageAmazonRDS,
	ConnectorName: postgres.ConnectorNameAmazonRDS,
})
