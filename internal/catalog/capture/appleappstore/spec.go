package appleappstore

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// Image is the Apple App Store connector image.
const Image string = "ghcr.io/estuary/source-apple-app-store:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "source-apple-app-store"

// Credentials is the App Store Connect API key the connector signs its requests with, whose only variant is
// native.CredentialsTitlePrivateApp. Its fields are all secrets, optional so that an import can leave them out.
type Credentials struct {
	Title       native.CredentialsTitle `yaml:"credentials_title" secret:"false" description:"Authentication method, which must be 'Private App Credentials', the only one the connector has."`
	PrivateKey  string                  `yaml:"private_key,omitempty" description:"Contents of the private key file (.p8) of the App Store Connect API key, with its BEGIN and END lines. The connector requires it; an imported capture can leave it out to keep the one Estuary holds. Write-only."`
	KeyID       string                  `yaml:"key_id,omitempty" secret:"true" description:"Key ID of the App Store Connect API key. The connector requires it; an imported capture can leave it out. Write-only."`
	IssuerID    string                  `yaml:"issuer_id,omitempty" secret:"true" description:"Issuer ID of the App Store Connect API keys, under Users and Access > Keys. The connector requires it; an imported capture can leave it out. Write-only."`
	AccessToken string                  `yaml:"access_token,omitempty" description:"Access token, which the dashboard hides and leaves empty. Leave it unset. Write-only."`
}

// Config is the source-apple-app-store connector configuration.
type Config struct {
	Credentials Credentials `yaml:"credentials" description:"App Store Connect API key the connector authenticates with."`
	AppIDs      []string    `yaml:"app_ids,omitempty" description:"IDs of the apps to capture. Unset, every app the API key can access is captured."`
}

// Resource identifies the captured App Store stream and its polling interval.
type Resource = native.Resource

// Binding maps a captured App Store stream to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Apple App Store capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
