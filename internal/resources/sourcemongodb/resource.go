package sourcemongodb

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/mongodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
)

var New = builder.NewCapture[mongodb.Config, mongodb.Resource](builder.Connector{
	TypeName:      "source_mongodb",
	Image:         mongodb.Image,
	ConnectorName: mongodb.ConnectorName,
})
