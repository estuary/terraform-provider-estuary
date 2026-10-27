package provider

import (
	"github.com/estuary/terraform-provider-estuary/internal/resources/materializationpostgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourceauroramysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourceaurorapostgres"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcedynamodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcemongodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcerdsmysql"
	"github.com/estuary/terraform-provider-estuary/internal/resources/sourcerdspostgres"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// resources lists every supported connector resource, sorted by Terraform type name.
var resources = []func() resource.Resource{
	materializationpostgres.New,
	sourceauroramysql.New,
	sourceaurorapostgres.New,
	sourcedynamodb.New,
	sourcemongodb.New,
	sourcerdsmysql.New,
	sourcerdspostgres.New,
}
