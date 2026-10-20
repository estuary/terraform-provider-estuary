package mongodb

import (
	spec "github.com/estuary/terraform-provider-estuary/internal/catalog/capture/mongodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
)

var New = builder.NewCapture[spec.Config, spec.Resource](builder.Connector{
	TypeName:      "source_mongodb",
	Image:         spec.Image,
	ConnectorName: spec.ConnectorName,
})
