package provider

import (
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationauroramysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationaurorapostgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationmysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationpostgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationrdsmysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationrdspostgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourceauroramysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourceaurorapostgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcedynamodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcemongodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcerdsmysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcerdspostgres"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// resources lists every supported connector resource, sorted by Terraform type name.
var resources []func() resource.Resource = []func() resource.Resource{
	materializationauroramysql.New,
	materializationaurorapostgres.New,
	materializationmysql.New,
	materializationpostgres.New,
	materializationrdsmysql.New,
	materializationrdspostgres.New,
	sourceauroramysql.New,
	sourceaurorapostgres.New,
	sourcedynamodb.New,
	sourcemongodb.New,
	sourcerdsmysql.New,
	sourcerdspostgres.New,
}

// dataSources lists every supported connector data source, sorted by Terraform type name.
var dataSources []func() datasource.DataSource = []func() datasource.DataSource{
	sourceauroramysql.NewDataSource,
	sourceaurorapostgres.NewDataSource,
	sourcedynamodb.NewDataSource,
	sourcemongodb.NewDataSource,
	sourcerdsmysql.NewDataSource,
	sourcerdspostgres.NewDataSource,
}
