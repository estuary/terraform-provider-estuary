package stripe

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// Image is the Stripe connector image.
const Image string = "ghcr.io/estuary/source-stripe-native:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "source-stripe-native"

// Credentials is the Stripe authentication, whose only variant is native.CredentialsTitlePrivateApp. AccessToken is
// optional so that an import can leave it out, like the secrets of the other native credentials blocks.
type Credentials struct {
	Title       native.CredentialsTitle `yaml:"credentials_title" secret:"false" description:"Authentication method, which must be 'Private App Credentials', the only one the connector has."`
	AccessToken string                  `yaml:"access_token,omitempty" description:"Stripe API key, a secret or restricted key (e.g., 'sk_live_...'). The connector requires it; an imported capture can leave it out to keep the key Estuary holds. Write-only."`
}

// Advanced holds the advanced endpoint options.
type Advanced struct {
	APIVersion            string `yaml:"api_version,omitempty" description:"Stripe API version of every request (e.g., '2024-06-20'). Defaults to the account's default version. Streams that need a newer version than the one in effect are disabled."`
	IncrementalWindowSize string `yaml:"incremental_window_size,omitempty" description:"Longest time span one incremental sweep processes, as an ISO 8601 duration such as 'PT1H'. It bounds the catch-up work done at once after many changes in a short time."`
}

// Config is the source-stripe-native connector configuration.
type Config struct {
	Credentials              Credentials `yaml:"credentials" description:"How the connector authenticates with Stripe."`
	StartDate                string      `yaml:"start_date,omitempty" description:"Captures only the data created at or after this UTC time, as YYYY-MM-DDTHH:MM:SSZ. Defaults to 30 days before the capture is created."`
	CaptureConnectedAccounts bool        `yaml:"capture_connected_accounts,omitempty" description:"Also captures the data of the accounts connected to the platform account. Defaults to false."`
	Advanced                 *Advanced   `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
}

// Resource identifies the captured Stripe stream, its polling interval and its scheduled backfill.
type Resource = native.ScheduledResource

// Binding maps a captured Stripe stream to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Stripe capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
