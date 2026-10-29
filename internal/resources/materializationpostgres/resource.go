package materializationpostgres

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/postgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// New returns the estuary_materialization_postgres resource.
var New func() resource.Resource = builder.NewMaterialization[postgres.Config, postgres.Resource](builder.Connector{
	TypeName:      "materialization_postgres",
	Image:         postgres.ImagePostgres,
	ConnectorName: postgres.ConnectorNamePostgres,
})
