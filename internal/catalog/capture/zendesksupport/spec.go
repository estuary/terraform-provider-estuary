package zendesksupport

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// Image is the Zendesk Support connector image.
const Image string = "ghcr.io/estuary/source-zendesk-support-native:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "source-zendesk-support-native"

// CredentialsTitleAPIToken is the variant authenticating with an agent's email and an API token.
const CredentialsTitleAPIToken native.CredentialsTitle = "Email & API Token"

// CredentialsTitles lists every supported credentials variant, useful for schema validators.
var CredentialsTitles = []native.CredentialsTitle{
	CredentialsTitleAPIToken,
	native.CredentialsTitleOAuth,
}

// Credentials is the Zendesk authentication, using the fields of its Title. The connector rewrites the OAuth tokens
// and AccessTokenExpiresAt each time it refreshes the access token, so AccessTokenExpiresAt is write-only like them,
// through its key: read back, each refresh would show as a change.
type Credentials struct {
	Title                native.CredentialsTitle `yaml:"credentials_title" secret:"false" enum:"Email & API Token,OAuth Credentials" description:"Authentication method: 'Email & API Token' (an agent's email and a Zendesk API token) or 'OAuth Credentials' (an OAuth client with the tokens it was granted, as the dashboard login stores). Only the attributes of the selected method are used."`
	Username             string                  `yaml:"username,omitempty" description:"Email of the Zendesk agent the API token belongs to, with 'Email & API Token'."`
	Password             string                  `yaml:"password,omitempty" description:"Zendesk API token, with 'Email & API Token'. Write-only."`
	native.OAuthClient   `yaml:",inline"`
	RefreshToken         string `yaml:"refresh_token,omitempty" description:"Refresh token the OAuth client was granted, with 'OAuth Credentials'. The connector replaces it each time it refreshes the access token. Leave it out of a capture authenticated through the dashboard login. Write-only."`
	AccessToken          string `yaml:"access_token,omitempty" description:"Access token the OAuth client was granted, with 'OAuth Credentials'. The connector replaces it when it expires. Leave it out of a capture authenticated through the dashboard login. Write-only."`
	AccessTokenExpiresAt string `yaml:"access_token_expires_at,omitempty" description:"UTC time access_token expires at, as YYYY-MM-DDTHH:MM:SSZ, with 'OAuth Credentials'. The connector replaces it with each new access token, so Terraform sets it and never reads it back. Leave it out of a capture authenticated through the dashboard login. Write-only."`
}

// Config is the source-zendesk-support-native connector configuration.
type Config struct {
	Subdomain   string      `yaml:"subdomain" description:"Subdomain of the Zendesk account: 'acme' for https://acme.zendesk.com."`
	StartDate   string      `yaml:"start_date,omitempty" description:"Captures only the data created or changed at or after this UTC time, as YYYY-MM-DDTHH:MM:SSZ. Defaults to 30 days before the capture is created."`
	Credentials Credentials `yaml:"credentials" description:"How the connector authenticates with Zendesk."`
}

// Resource identifies the captured Zendesk Support stream and its polling interval.
type Resource = native.Resource

// Binding maps a captured Zendesk Support stream to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Zendesk Support capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
