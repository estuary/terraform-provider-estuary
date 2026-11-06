package materializationsnowflake

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/snowflake"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// New returns the estuary_materialization_snowflake resource.
var New func() resource.Resource = builder.NewMaterialization[snowflake.Config, snowflake.Resource](builder.Connector{
	TypeName:      "materialization_snowflake",
	Image:         snowflake.Image,
	ConnectorName: snowflake.ConnectorName,
	Description:   "Materializes Estuary collections into Snowflake tables.",
})
