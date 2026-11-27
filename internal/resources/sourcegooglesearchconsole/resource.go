package sourcegooglesearchconsole

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/googlesearchconsole"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var connector builder.Connector = builder.Connector{
	TypeName:      "source_google_search_console",
	Image:         googlesearchconsole.Image,
	ConnectorName: googlesearchconsole.ConnectorName,
	Description:   "Captures the search analytics, sitemaps and sites of Google Search Console properties into Estuary collections.",
}

// New returns the estuary_source_google_search_console resource.
var New func() resource.Resource = builder.NewCapture[googlesearchconsole.Config, googlesearchconsole.Resource](connector)

// NewDataSource returns the estuary_source_google_search_console data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[googlesearchconsole.Config, googlesearchconsole.Resource](connector)
