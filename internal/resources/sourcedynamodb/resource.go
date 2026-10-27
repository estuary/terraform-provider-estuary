package sourcedynamodb

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/amazon/dynamodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
)

var New = builder.NewCapture[dynamodb.Config, dynamodb.Resource](builder.Connector{
	TypeName:      "source_dynamodb",
	Image:         dynamodb.Image,
	ConnectorName: dynamodb.ConnectorName,
})
