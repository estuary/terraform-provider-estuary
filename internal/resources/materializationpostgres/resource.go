package materializationpostgres

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/postgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
)

var New = builder.NewMaterialization[postgres.Config, postgres.Resource](builder.Connector{
	TypeName:      "materialization_postgres",
	Image:         postgres.Image,
	ConnectorName: postgres.ConnectorName,
})
