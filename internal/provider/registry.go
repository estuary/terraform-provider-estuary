package provider

import (
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationdatabricks"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationdekaf"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationiceberg"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationmysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationpostgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializations3iceberg"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationsnowflake"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationwebhook"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourceappleappstore"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcedynamodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcefacebookmarketing"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcegoogleplay"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcemongodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcemysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcepostgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcesalesforce"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcestripe"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcezendesksupport"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// resources lists every supported connector resource, sorted by Terraform type name.
var resources []func() resource.Resource = []func() resource.Resource{
	materializationmysql.NewAurora,
	materializationpostgres.NewAurora,
	materializationdatabricks.New,
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
	materializationsnowflake.New,
	materializationwebhook.New,
	sourceappleappstore.New,
	sourcemysql.NewAurora,
	sourcepostgres.NewAurora,
	sourcedynamodb.New,
	sourcefacebookmarketing.New,
	sourcegoogleplay.New,
	sourcemongodb.New,
	sourcemysql.New,
	sourcepostgres.New,
	sourcemysql.NewRDS,
	sourcepostgres.NewRDS,
	sourcesalesforce.New,
	sourcestripe.New,
	sourcezendesksupport.New,
}

// dataSources lists every supported connector data source, sorted by Terraform type name.
var dataSources []func() datasource.DataSource = []func() datasource.DataSource{
	sourceappleappstore.NewDataSource,
	sourcemysql.NewAuroraDataSource,
	sourcepostgres.NewAuroraDataSource,
	sourcedynamodb.NewDataSource,
	sourcefacebookmarketing.NewDataSource,
	sourcegoogleplay.NewDataSource,
	sourcemongodb.NewDataSource,
	sourcemysql.NewDataSource,
	sourcepostgres.NewDataSource,
	sourcemysql.NewRDSDataSource,
	sourcepostgres.NewRDSDataSource,
	sourcesalesforce.NewDataSource,
	sourcestripe.NewDataSource,
	sourcezendesksupport.NewDataSource,
}
