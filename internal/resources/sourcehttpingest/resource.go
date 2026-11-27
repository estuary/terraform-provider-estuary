package sourcehttpingest

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/httpingest"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var connector builder.Connector = builder.Connector{
	TypeName:      "source_http_ingest",
	Image:         httpingest.Image,
	ConnectorName: httpingest.ConnectorName,
	Description:   "Captures the JSON documents sent to an HTTP endpoint Estuary hosts, such as webhook deliveries, into Estuary collections.",
}

// New returns the estuary_source_http_ingest resource.
var New func() resource.Resource = builder.NewCapture[httpingest.Config, httpingest.Resource](connector)

// NewDataSource returns the estuary_source_http_ingest data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[httpingest.Config, httpingest.Resource](connector)
