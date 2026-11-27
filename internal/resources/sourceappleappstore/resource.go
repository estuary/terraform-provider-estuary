package sourceappleappstore

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/appleappstore"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var connector builder.Connector = builder.Connector{
	TypeName:      "source_apple_app_store",
	Image:         appleappstore.Image,
	ConnectorName: appleappstore.ConnectorName,
	Description:   "Captures the customer reviews and analytics reports of App Store apps into Estuary collections, through the App Store Connect API.",
}

// New returns the estuary_source_apple_app_store resource.
var New func() resource.Resource = builder.NewCapture[appleappstore.Config, appleappstore.Resource](connector)

// NewDataSource returns the estuary_source_apple_app_store data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[appleappstore.Config, appleappstore.Resource](connector)
