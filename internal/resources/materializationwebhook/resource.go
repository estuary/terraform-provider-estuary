package materializationwebhook

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/webhook"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// New returns the estuary_materialization_webhook resource.
var New func() resource.Resource = builder.NewMaterialization[webhook.Config, webhook.Resource](builder.Connector{
	TypeName:      "materialization_webhook",
	Image:         webhook.Image,
	ConnectorName: webhook.ConnectorName,
	Description:   "Sends the documents of Estuary collections to an HTTP endpoint of your server.",
})
