package googleplay

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// Image is the Google Play connector image.
const Image string = "ghcr.io/estuary/source-google-play:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "source-google-play"

// CredentialsTitleServiceAccount is the variant authenticating with the JSON key of a Google service account.
const CredentialsTitleServiceAccount native.CredentialsTitle = "Google Service Account"

// CredentialsTitles lists every supported credentials variant, useful for schema validators.
var CredentialsTitles = []native.CredentialsTitle{
	CredentialsTitleServiceAccount,
}

// Credentials is the Google Play authentication, whose only variant is CredentialsTitleServiceAccount. ServiceAccount
// is optional so that an import can leave it out, like the secrets of the other native credentials blocks.
type Credentials struct {
	Title          native.CredentialsTitle `yaml:"credentials_title" secret:"false" enum:"Google Service Account" description:"Authentication method, which must be 'Google Service Account', the only one the connector has."`
	ServiceAccount string                  `yaml:"service_account,omitempty" secret:"true" description:"JSON key of the Google service account invited to the Play Console. The connector requires it; an imported capture can leave it out to keep the key Estuary holds. Write-only."`
}

// Config is the source-google-play connector configuration.
type Config struct {
	Bucket      string      `yaml:"bucket" description:"Cloud Storage bucket holding the Google Play reports of the developer account, which starts with pubsite_prod (e.g., 'pubsite_prod_1234567890123456789')."`
	StartDate   string      `yaml:"start_date,omitempty" description:"Captures only the data at or after this UTC time, as YYYY-MM-DDTHH:MM:SSZ. Defaults to 30 days before the capture is created."`
	Credentials Credentials `yaml:"credentials" description:"How the connector authenticates with Google."`
}

// Resource identifies the captured Google Play report and its polling interval.
type Resource = native.Resource

// Binding maps a captured Google Play report to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Google Play capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
