package sourcemongodb

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/mongodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var connector builder.Connector = builder.Connector{
	TypeName:      "source_mongodb",
	Image:         mongodb.Image,
	ConnectorName: mongodb.ConnectorName,
	Description:   "Captures MongoDB collections into Estuary collections, through change streams or batch polling.",
}

// New returns the estuary_source_mongodb resource.
var New func() resource.Resource = builder.NewCapture[mongodb.Config, mongodb.Resource](connector)

// NewDataSource returns the estuary_source_mongodb data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[mongodb.Config, mongodb.Resource](connector)
