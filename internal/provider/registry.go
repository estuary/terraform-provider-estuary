package provider

import (
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationdekaf"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationiceberg"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationmysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationpostgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializations3iceberg"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcedynamodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcemongodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcemysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcepostgres"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// resources lists every supported connector resource, sorted by Terraform type name.
var resources []func() resource.Resource = []func() resource.Resource{
	materializationmysql.NewAurora,
	materializationpostgres.NewAurora,
	materializationdekaf.NewBytewax,
	materializationdekaf.NewClickHouse,
	materializationdekaf.NewGeneric,
	materializationdekaf.NewImplyPolaris,
	materializationdekaf.NewMaterialize,
	materializationdekaf.NewRisingWave,
	materializationdekaf.NewSingleStore,
	materializationdekaf.NewStarTree,
	materializationdekaf.NewTinybird,
	materializationiceberg.New,
	materializationmysql.New,
	materializationpostgres.New,
	materializationmysql.NewRDS,
	materializationpostgres.NewRDS,
	materializations3iceberg.New,
	sourcemysql.NewAurora,
	sourcepostgres.NewAurora,
	sourcedynamodb.New,
	sourcemongodb.New,
	sourcemysql.NewRDS,
	sourcepostgres.NewRDS,
}

// dataSources lists every supported connector data source, sorted by Terraform type name.
var dataSources []func() datasource.DataSource = []func() datasource.DataSource{
	sourcemysql.NewAuroraDataSource,
	sourcepostgres.NewAuroraDataSource,
	sourcedynamodb.NewDataSource,
	sourcemongodb.NewDataSource,
	sourcemysql.NewRDSDataSource,
	sourcepostgres.NewRDSDataSource,
}
