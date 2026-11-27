package salesforce

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// WindowType is the unit the incremental query window is sized in.
type WindowType string

// Image is the Salesforce connector image.
const Image string = "ghcr.io/estuary/source-salesforce-native:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "source-salesforce-native"

// CredentialsTitleClientCredentials is the variant authenticating through the OAuth client credentials flow.
const CredentialsTitleClientCredentials native.CredentialsTitle = "Client Credentials"

// CredentialsTitles lists every supported credentials variant, useful for schema validators.
var CredentialsTitles = []native.CredentialsTitle{
	CredentialsTitleClientCredentials,
	native.CredentialsTitleOAuth,
}

const (
	WindowTypeInterval WindowType = "interval"
	WindowTypeDays     WindowType = "days"
)

// WindowTypes lists every supported window type, useful for schema validators.
var WindowTypes = []WindowType{
	WindowTypeInterval,
	WindowTypeDays,
}

// Credentials is the Salesforce authentication, using the fields of its Title.
type Credentials struct {
	Title              native.CredentialsTitle `yaml:"credentials_title" secret:"false" enum:"Client Credentials,OAuth Credentials" description:"Authentication method: 'Client Credentials' (the consumer key and secret of your connected app or external client app, through the client credentials flow) or 'OAuth Credentials' (an OAuth app with a refresh token, as the dashboard login stores). Only the attributes of the selected method are used."`
	native.OAuthClient `yaml:",inline"`
	RefreshToken       string `yaml:"refresh_token,omitempty" description:"Refresh token the OAuth app was granted, with 'OAuth Credentials'. Leave it out of a capture authenticated through the dashboard login. Write-only."`
	InstanceURL        string `yaml:"instance_url,omitempty" description:"URL of the Salesforce instance the refresh token belongs to (e.g., 'https://acme.my.salesforce.com'), with 'OAuth Credentials'."`
}

// WindowSize is how much time one incremental query covers, in the unit of its WindowType.
type WindowSize struct {
	WindowType WindowType `yaml:"window_type" enum:"interval,days" description:"Unit of the window: 'interval' sizes it with interval, 'days' with days."`
	Interval   string     `yaml:"interval,omitempty" description:"Size of the window as an ISO 8601 duration such as 'PT1H', with 'interval'."`
	Days       int        `yaml:"days,omitempty" description:"Size of the window in whole days, at least 1, with 'days'."`
}

// Advanced holds the advanced endpoint options.
type Advanced struct {
	WindowSize *WindowSize `yaml:"window_size,omitempty" description:"How much time one incremental query covers when the connector sweeps for changes. Leave it unset unless Estuary support or the connector logs say otherwise."`
}

// Config is the source-salesforce-native connector configuration.
type Config struct {
	Credentials Credentials `yaml:"credentials" description:"How the connector authenticates with Salesforce."`
	StartDate   string      `yaml:"start_date,omitempty" description:"Captures only the records created or changed at or after this UTC time, as YYYY-MM-DDTHH:MM:SSZ. Defaults to 1999-02-03T00:00:00Z, Salesforce's founding date."`
	IsSandbox   bool        `yaml:"is_sandbox,omitempty" description:"Whether the org is a Salesforce sandbox. Defaults to false."`
	MyDomain    string      `yaml:"my_domain,omitempty" description:"My Domain login host of the org, ending in .my.salesforce.com (e.g., 'acme.my.salesforce.com' or 'acme--uat.sandbox.my.salesforce.com'). Required with 'Client Credentials'. Unset, the connector logs in through the standard login or test endpoint."`
	Advanced    *Advanced   `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
}

// Resource identifies the captured Salesforce object, its polling interval and its formula field refresh schedule.
type Resource = native.ScheduledResource

// Binding maps a captured Salesforce object to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Salesforce capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
