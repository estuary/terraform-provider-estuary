package postgres

import (
	spec "github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/postgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
)

var New = builder.NewMaterialization[spec.Config, spec.Resource](builder.Connector{
	TypeName:      "materialization_postgres",
	Image:         spec.Image,
	ConnectorName: spec.ConnectorName,
})
