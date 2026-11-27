// Package native holds the blocks the Estuary CDK ("native") sources share: the parts of their credentials block and
// their binding resource.
package native

// CredentialsTitle selects the variant of a native source's credentials block, through its credentials_title key.
// Each connector lists the variants it offers.
type CredentialsTitle string

// CredentialsTitleOAuth is the variant holding the credentials of an OAuth app.
const CredentialsTitleOAuth CredentialsTitle = "OAuth Credentials"

// OAuthClient is the OAuth app of a credentials variant. Both fields are optional, so an import can leave out the app
// of a dashboard login, which is Estuary's.
type OAuthClient struct {
	ClientID     string `yaml:"client_id,omitempty" secret:"true" description:"Client ID of the OAuth app the connector authenticates as, which some services call a consumer key. Leave it out of a capture authenticated through the dashboard login, whose OAuth app is Estuary's. Write-only."`
	ClientSecret string `yaml:"client_secret,omitempty" description:"Client secret of the OAuth app, which some services call a consumer secret. Leave it out of a capture authenticated through the dashboard login. Write-only."`
}

// Resource identifies the stream a native source binding captures and how often it's polled.
type Resource struct {
	Name     string `yaml:"name" description:"Name of the captured stream, as the connector discovery and the dashboard list it."`
	Interval string `yaml:"interval,omitempty" description:"Time between two polls of the stream, as an ISO 8601 duration such as 'PT5M'. Set it to the interval discovery gives the stream, as the dashboard shows: omitted, the connector polls again right away ('PT0S')."`
}

// ScheduledResource is the Resource of the native sources whose streams also run work on a cron schedule.
type ScheduledResource struct {
	Resource `yaml:",inline"`
	Schedule string `yaml:"schedule,omitempty" description:"Cron expression of the scheduled work the connector runs for the stream, in UTC with five fields (e.g., '55 23 * * *'). What runs depends on the connector, see its documentation. Omitted, nothing is scheduled."`
}

// SnapshotResource is the binding resource R as native connector schemas declare it: with the _meta key the control
// plane fills on live bindings. The builder carries _meta over, so it isn't an attribute; spec tests check this type
// against their schema snapshot.
type SnapshotResource[R any] struct {
	Resource R              `yaml:",inline"`
	Meta     map[string]any `yaml:"_meta,omitempty"`
}

// ResourcePath returns the stream name, satisfying capture.ResourcePather.
func (resource Resource) ResourcePath() []string {
	return []string{resource.Name}
}
