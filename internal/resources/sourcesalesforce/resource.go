package sourcesalesforce

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/salesforce"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var connector builder.Connector = builder.Connector{
	TypeName:      "source_salesforce",
	Image:         salesforce.Image,
	ConnectorName: salesforce.ConnectorName,
	Description:   "Captures Salesforce objects into Estuary collections, backfilling through the Bulk API and polling the REST API for changes.",
}

// New returns the estuary_source_salesforce resource.
var New func() resource.Resource = builder.NewCapture[salesforce.Config, salesforce.Resource](connector)

// NewDataSource returns the estuary_source_salesforce data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[salesforce.Config, salesforce.Resource](connector)
