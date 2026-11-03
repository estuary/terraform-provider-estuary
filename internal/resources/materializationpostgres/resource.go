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
	Description:   "Materializes Estuary collections into tables of a PostgreSQL-compatible database, such as Google Cloud SQL.",
})

// NewAurora returns the estuary_materialization_aurora_postgres resource.
var NewAurora func() resource.Resource = builder.NewMaterialization[postgres.Config, postgres.Resource](builder.Connector{
	TypeName:      "materialization_aurora_postgres",
	Image:         postgres.ImageAmazonAurora,
	ConnectorName: postgres.ConnectorNameAmazonAurora,
	Description:   "Materializes Estuary collections into tables of an Amazon Aurora for PostgreSQL database.",
})

// NewRDS returns the estuary_materialization_rds_postgres resource.
var NewRDS func() resource.Resource = builder.NewMaterialization[postgres.Config, postgres.Resource](builder.Connector{
	TypeName:      "materialization_rds_postgres",
	Image:         postgres.ImageAmazonRDS,
	ConnectorName: postgres.ConnectorNameAmazonRDS,
	Description:   "Materializes Estuary collections into tables of an Amazon RDS for PostgreSQL database.",
})
