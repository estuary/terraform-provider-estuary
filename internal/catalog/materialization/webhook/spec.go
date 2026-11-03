package webhook

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
)

// Image is the HTTP Webhook connector image.
const Image string = "ghcr.io/estuary/materialize-webhook:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "materialize-webhook"

// CustomHeader is an HTTP header added to every request.
type CustomHeader struct {
	Name  string `yaml:"name" description:"Name of the header (e.g., 'Authorization'). It can't be empty."`
	Value string `yaml:"value" secret:"true" description:"Value of the header, such as a token, as headers usually carry credentials. It can't be empty. Write-only."`
}

// Headers holds the custom HTTP headers.
type Headers struct {
	CustomHeaders []CustomHeader `yaml:"customHeaders,omitempty" description:"Additional headers included in every request."`
}

// Advanced holds advanced endpoint options.
type Advanced struct {
	FeatureFlags string `yaml:"feature_flags,omitempty" description:"Connector settings for edge cases and experimental features, see https://docs.estuary.dev/guides/advanced-usage/feature-flags/. Intended for Estuary internal use: change them only as directed by Estuary support."`
}

// Config is the materialize-webhook connector configuration.
type Config struct {
	Address  string    `yaml:"address" credentials:"forbidden" description:"Base URL the requests are sent to, ending in a trailing '/' (e.g., 'https://hooks.example.com/estuary/'). The plan rejects credentials in it, such as user:password@ or a token query option, as state and logs would hold them in plain text: send them in a header instead. Credentials found in the live value show as ***."`
	Headers  *Headers  `yaml:"headers,omitempty" description:"HTTP headers of the requests."`
	Advanced *Advanced `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
}

// Resource is the path, joined with Config.Address, that a binding's documents are posted to.
type Resource struct {
	RelativePath string `yaml:"relativePath" credentials:"forbidden" description:"Path joined with address to build the URL the binding's documents are posted to, as a relative URL reference (e.g., 'orders' or '/orders'). The plan rejects credentials in it, as in address."`
}

// Binding maps a source collection to its webhook URL.
type Binding = materialization.Binding[Resource]

// EndpointSpec wraps the connector under endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is an HTTP Webhook materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]
