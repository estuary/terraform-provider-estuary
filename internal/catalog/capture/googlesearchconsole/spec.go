package googlesearchconsole

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// AuthType identifies how the connector authenticates with Google.
type AuthType string

// SyncMode is how a stream is captured: its changes since the last sync, or all of it each time.
type SyncMode string

// Image is the Google Search Console connector image.
const Image string = "ghcr.io/estuary/source-google-search-console:v2"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "source-google-search-console"

const (
	AuthTypeClient  AuthType = "Client"
	AuthTypeService AuthType = "Service"
)

// AuthTypes lists every supported auth type, useful for schema validators.
var AuthTypes = []AuthType{
	AuthTypeClient,
	AuthTypeService,
}

const (
	SyncModeIncremental SyncMode = "incremental"
	SyncModeFullRefresh SyncMode = "full_refresh"
)

// SyncModes lists every supported sync mode, useful for schema validators.
var SyncModes = []SyncMode{
	SyncModeIncremental,
	SyncModeFullRefresh,
}

// Credentials is the Google authentication, with the fields of its AuthType. The OAuth fields are optional so that an
// import can leave out the client of a dashboard login, which is Estuary's.
type Credentials struct {
	AuthType           AuthType `yaml:"auth_type" enum:"Client,Service" description:"Authentication method: 'Client' (an OAuth client with a refresh token, as the dashboard login stores) or 'Service' (the JSON key of a Google service account). Only the attributes of the selected method are used."`
	ClientID           string   `yaml:"client_id,omitempty" secret:"true" description:"Client ID of the OAuth client the connector authenticates as, with 'Client'. Leave it out of a capture authenticated through the dashboard login, whose OAuth client is Estuary's. Write-only."`
	ClientSecret       string   `yaml:"client_secret,omitempty" description:"Client secret of the OAuth client, with 'Client'. Leave it out of a capture authenticated through the dashboard login. Write-only."`
	RefreshToken       string   `yaml:"refresh_token,omitempty" description:"Refresh token the OAuth client was granted, with 'Client'. Leave it out of a capture authenticated through the dashboard login. Write-only."`
	ServiceAccountInfo string   `yaml:"service_account_info,omitempty" secret:"true" description:"JSON key of the Google service account, with 'Service'. Write-only."`
	Email              string   `yaml:"email,omitempty" description:"Email of the Google Workspace user the service account acts as, through domain-wide delegation, with 'Service'."`
}

// Config is the source-google-search-console connector configuration.
type Config struct {
	SiteURLs      []string    `yaml:"site_urls" description:"Search Console properties to capture, as their URL (e.g., 'https://example.com/') or domain property (e.g., 'sc-domain:example.com')."`
	StartDate     string      `yaml:"start_date" description:"Captures only the data at or after this UTC date, as YYYY-MM-DD."`
	EndDate       string      `yaml:"end_date,omitempty" description:"Captures only the data at or before this UTC date, as YYYY-MM-DD, at or after start_date. Unset, the capture keeps following new data."`
	CustomReports string      `yaml:"custom_reports,omitempty" description:"Custom reports captured besides the built-in ones, as a JSON array of objects with a name and its dimensions (e.g., '[{\"name\": \"by_country_and_device\", \"dimensions\": [\"date\", \"country\", \"device\"]}]'). Each report becomes a stream."`
	Credentials   Credentials `yaml:"credentials" description:"How the connector authenticates with Google."`
}

// Resource identifies the captured Search Console stream and how it is synced.
type Resource struct {
	Stream      string   `yaml:"stream" description:"Name of the captured stream, as the connector discovery and the dashboard list it (e.g., 'search_analytics_by_date')."`
	SyncMode    SyncMode `yaml:"syncMode" enum:"incremental,full_refresh" description:"How the stream is captured: 'incremental' reads what changed since the last sync, 'full_refresh' reads all of it each time. Use the mode discovery gives the stream."`
	Namespace   string   `yaml:"namespace,omitempty" description:"Namespace of the stream. The streams of this connector have none."`
	CursorField []string `yaml:"cursorField,omitempty" description:"Path of the field an incremental stream is ordered by (e.g., ['date']). Unset, the stream's own default cursor is used."`
}

// ResourcePath returns the stream, after its namespace when it has one, satisfying capture.ResourcePather.
func (resource Resource) ResourcePath() []string {
	if resource.Namespace == "" {
		return []string{resource.Stream}
	}

	return []string{resource.Namespace, resource.Stream}
}

// Binding maps a captured Search Console stream to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Google Search Console capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
