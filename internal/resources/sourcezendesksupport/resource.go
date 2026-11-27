package sourcezendesksupport

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/zendesksupport"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// connector rotates its secrets: with OAuth credentials, the connector replaces its access and refresh tokens.
var connector builder.Connector = builder.Connector{
	TypeName:       "source_zendesk_support",
	Image:          zendesksupport.Image,
	ConnectorName:  zendesksupport.ConnectorName,
	Description:    "Captures Zendesk Support resources, such as tickets, users and organizations, into Estuary collections.",
	RotatesSecrets: true,
}

// New returns the estuary_source_zendesk_support resource.
var New func() resource.Resource = builder.NewCapture[zendesksupport.Config, zendesksupport.Resource](connector)

// NewDataSource returns the estuary_source_zendesk_support data source, reading the live bindings of a capture by name.
var NewDataSource func() datasource.DataSource = builder.NewCaptureDataSource[zendesksupport.Config, zendesksupport.Resource](connector)
