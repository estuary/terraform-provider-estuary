package sourcegoogleplay

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/googleplay"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var connector builder.Connector = builder.Connector{
	TypeName:      "source_google_play",
	Image:         googleplay.Image,
	ConnectorName: googleplay.ConnectorName,
	Description:   "Captures the Google Play reports of a developer account, such as installs, crashes and reviews, into Estuary collections.",
}

// New returns the estuary_source_google_play resource.
var New func() resource.Resource = builder.NewCapture[googleplay.Config, googleplay.Resource](connector)

// NewDataSource returns the estuary_source_google_play data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[googleplay.Config, googleplay.Resource](connector)
