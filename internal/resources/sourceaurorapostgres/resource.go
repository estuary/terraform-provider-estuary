package sourceaurorapostgres

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/postgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
)

var New = builder.NewCapture[postgres.Config, postgres.Resource](builder.Connector{
	TypeName:      "source_aurora_postgres",
	Image:         postgres.ImageAmazonAurora,
	ConnectorName: postgres.ConnectorNameAmazonAurora,
})
