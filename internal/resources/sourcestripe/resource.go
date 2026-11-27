package sourcestripe

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/stripe"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var connector builder.Connector = builder.Connector{
	TypeName:      "source_stripe",
	Image:         stripe.Image,
	ConnectorName: stripe.ConnectorName,
	Description:   "Captures Stripe resources, such as charges, customers and invoices, into Estuary collections, in near real time through the Stripe Events API.",
}

// New returns the estuary_source_stripe resource.
var New func() resource.Resource = builder.NewCapture[stripe.Config, stripe.Resource](connector)

// NewDataSource returns the estuary_source_stripe data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[stripe.Config, stripe.Resource](connector)
