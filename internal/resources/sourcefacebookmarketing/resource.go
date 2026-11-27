package sourcefacebookmarketing

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/facebookmarketing"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var connector builder.Connector = builder.Connector{
	TypeName:      "source_facebook_marketing",
	Image:         facebookmarketing.Image,
	ConnectorName: facebookmarketing.ConnectorName,
	Description:   "Captures Facebook Marketing resources, such as campaigns, ads and their insights, into Estuary collections.",
}

// New returns the estuary_source_facebook_marketing resource.
var New func() resource.Resource = builder.NewCapture[facebookmarketing.Config, facebookmarketing.Resource](connector)

// NewDataSource returns the estuary_source_facebook_marketing data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[facebookmarketing.Config, facebookmarketing.Resource](connector)
