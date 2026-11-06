package sourcedynamodb

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/amazon/dynamodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var connector builder.Connector = builder.Connector{
	TypeName:      "source_dynamodb",
	Image:         dynamodb.Image,
	ConnectorName: dynamodb.ConnectorName,
	Description:   "Captures Amazon DynamoDB tables into Estuary collections, backfilling each table with parallel scans and reading its changes from DynamoDB Streams.",
}

// New returns the estuary_source_dynamodb resource.
var New func() resource.Resource = builder.NewCapture[dynamodb.Config, dynamodb.Resource](connector)

// NewDataSource returns the estuary_source_dynamodb data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[dynamodb.Config, dynamodb.Resource](connector)
