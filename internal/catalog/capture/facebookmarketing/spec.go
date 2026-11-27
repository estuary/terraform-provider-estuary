package facebookmarketing

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/native"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
)

// InsightLevel is the level of the ad hierarchy an insights report aggregates at.
type InsightLevel string

// Image is the Facebook Marketing connector image.
const Image string = "ghcr.io/estuary/source-facebook-marketing-native:v1"

// ConnectorName is the catalog name suffix of the connector's tasks, as the dashboard names them.
const ConnectorName string = "source-facebook-marketing-native"

const (
	InsightLevelAd       InsightLevel = "ad"
	InsightLevelAdSet    InsightLevel = "adset"
	InsightLevelCampaign InsightLevel = "campaign"
	InsightLevelAccount  InsightLevel = "account"
)

// InsightLevels lists every supported insight level, useful for schema validators.
var InsightLevels = []InsightLevel{
	InsightLevelAd,
	InsightLevelAdSet,
	InsightLevelCampaign,
	InsightLevelAccount,
}

// CredentialsTitles lists every supported credentials variant, useful for schema validators.
var CredentialsTitles = []native.CredentialsTitle{
	native.CredentialsTitleOAuth,
}

// Credentials is the Facebook authentication, whose only variant is native.CredentialsTitleOAuth. The connector sends
// only AccessToken to Facebook.
type Credentials struct {
	Title              native.CredentialsTitle `yaml:"credentials_title" secret:"false" enum:"OAuth Credentials" description:"Authentication method, which must be 'OAuth Credentials', the only one the connector has."`
	native.OAuthClient `yaml:",inline"`
	AccessToken        string `yaml:"access_token,omitempty" description:"Long-lived access token the connector sends to Facebook, such as a system user's or one of your own Meta app. Leave it out of a capture authenticated through the dashboard login. Write-only."`
}

// CustomInsight is an insights report captured as a stream of its own, named Name.
type CustomInsight struct {
	Name                   string       `yaml:"name" description:"Name of the report, which names its stream."`
	Level                  InsightLevel `yaml:"level,omitempty" enum:"ad,adset,campaign,account" description:"Level the report aggregates at: 'ad' (default), 'adset', 'campaign' or 'account'."`
	Fields                 string       `yaml:"fields" description:"Comma-separated fields of the report (e.g., 'account_id,ad_id,impressions,clicks,spend')."`
	Breakdowns             string       `yaml:"breakdowns" description:"Comma-separated breakdowns of the report (e.g., 'age,gender,region'), empty for none."`
	ActionBreakdowns       string       `yaml:"action_breakdowns,omitempty" description:"Comma-separated action breakdowns of the report (e.g., 'action_type')."`
	StartDate              string       `yaml:"start_date,omitempty" description:"Captures only the report data at or after this UTC time, as YYYY-MM-DDTHH:MM:SSZ."`
	InsightsLookbackWindow int          `yaml:"insights_lookback_window,omitempty" description:"Days of past report data the connector captures again, at least 1, as Facebook keeps changing insights for that long. Defaults to 28."`
}

// Advanced holds the advanced endpoint options.
type Advanced struct {
	IncludeDeleted       bool   `yaml:"include_deleted,omitempty" description:"Also captures the deleted entities of the resources that support it. Defaults to false."`
	FetchThumbnailImages bool   `yaml:"fetch_thumbnail_images,omitempty" description:"Fetches the thumbnail_url of each ad creative and stores the image in thumbnail_data_url. Defaults to false."`
	FeatureFlags         string `yaml:"feature_flags,omitempty" description:"Comma-separated experimental feature flags to enable. Change them only as directed by Estuary support."`
}

// Config is the source-facebook-marketing-native connector configuration.
type Config struct {
	AccountIDs             string          `yaml:"account_ids" description:"Comma-separated IDs of the Facebook ad accounts to capture (e.g., '123456789,987654321')."`
	StartDate              string          `yaml:"start_date,omitempty" description:"Captures only the data at or after this UTC time, as YYYY-MM-DDTHH:MM:SSZ."`
	InsightsLookbackWindow int             `yaml:"insights_lookback_window,omitempty" description:"Days of past insights the connector captures again, at least 1, as Facebook keeps changing insights for that long. Set it to the attribution window of the Facebook account when it has a custom one. Defaults to 28."`
	CustomInsights         []CustomInsight `yaml:"custom_insights,omitempty" description:"Insights reports captured besides the built-in ones, each as a stream of its own."`
	Advanced               *Advanced       `yaml:"advanced,omitempty" description:"Options for advanced users, rarely needed."`
	Credentials            Credentials     `yaml:"credentials" description:"How the connector authenticates with Facebook."`
}

// Resource identifies the captured Facebook Marketing stream and its polling interval.
type Resource = native.Resource

// Binding maps a captured Facebook Marketing stream to the collection it is captured into.
type Binding = capture.Binding[Resource]

// EndpointSpec wraps the connector under the "connector" key, as in endpoint.connector.
type EndpointSpec = endpoint.ConnectorSpec[Config]

// Spec is a full Facebook Marketing capture entry, keyed by its catalog name in a catalog spec.
type Spec = capture.Spec[EndpointSpec, Resource]
