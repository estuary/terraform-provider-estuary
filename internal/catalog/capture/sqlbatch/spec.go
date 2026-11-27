// Package sqlbatch holds the blocks the batch SQL captures share: their connection, SSH tunnel, common advanced
// options and binding resource. These connectors poll queries, unlike the change data captures of sqlcapture.
package sqlbatch

import "github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"

// Connection is the database server and the user the connector queries it as. Password is optional so that an import
// can leave it out and keep the sealed one.
type Connection struct {
	Address  string `yaml:"address" credentials:"forbidden" description:"Host of the database server, as host or host:port (e.g., 'db.example.com:3306'). The plan rejects credentials in it, such as user:password@ or a password option, as state and logs would hold them in plain text: set them in user and password. Credentials found in the live value show as ***."`
	User     string `yaml:"user" description:"Database user to connect as. The connector's own default is 'flow_capture'."`
	Password string `yaml:"password,omitempty" description:"Password of the database user. The connector requires it; an imported capture can leave it out to keep the password Estuary holds. Write-only."`
}

// SSHForwarding is the shared SSH tunnel endpoint, plus the host key settings of the batch connectors' schema.
type SSHForwarding struct {
	endpoint.SSHForwarding  `yaml:",inline"`
	KnownHosts              string `yaml:"knownHosts,omitempty" description:"Host keys of the SSH server the connector verifies it against, in OpenSSH known_hosts format, one per line, as ssh-keyscan prints them (e.g., '[bastion.example.com]:2222 ssh-ed25519 AAAA...')."`
	SkipHostKeyVerification bool   `yaml:"skipHostKeyVerification,omitempty" description:"Connects without verifying the host key of the SSH server. The dashboard hides it: prefer known_hosts."`
}

// NetworkTunnel connects to the database through an SSH server acting as a bastion host.
type NetworkTunnel struct {
	SSHForwarding *SSHForwarding `yaml:"sshForwarding,omitempty" description:"SSH server tunneling the connection to the database."`
}

// Advanced holds the advanced endpoint options every batch SQL capture has, inlined in each connector's own.
type Advanced struct {
	Poll            string   `yaml:"poll,omitempty" description:"Default polling schedule of the bindings: a Go duration such as '5m' or '6h', or a daily time in UTC such as 'daily at 12:34Z'. Defaults to '24h'."`
	DiscoverSchemas []string `yaml:"discover_schemas,omitempty" description:"Schemas whose tables are discovered. Unset, the tables of every schema are discovered."`
	DiscoverViews   bool     `yaml:"discover_views,omitempty" description:"Also discovers views. Otherwise only tables are discovered."`
	SourceTag       string   `yaml:"source_tag,omitempty" description:"Value added as the 'tag' property of the source metadata of each document."`
	FeatureFlags    string   `yaml:"feature_flags,omitempty" description:"Connector settings for edge cases and experimental features, see https://docs.estuary.dev/guides/advanced-usage/feature-flags/. Intended for Estuary internal use: change them only as directed by Estuary support."`
}

// Resource is the query a batch SQL binding polls: a table, optionally with cursor columns, or a custom template.
type Resource struct {
	Name     string   `yaml:"name" description:"Unique name of the binding in the capture, which discovery sets to the table name."`
	Schema   string   `yaml:"schema,omitempty" description:"Schema of the captured table. Required unless template is set."`
	Table    string   `yaml:"table,omitempty" description:"Name of the captured table. Required unless template is set."`
	Cursor   []string `yaml:"cursor,omitempty" description:"Columns whose last values are kept between polls, so each poll only reads the rows after them (e.g., an updated_at column or an incrementing ID). Unset, each poll reads the whole table again."`
	Poll     string   `yaml:"poll,omitempty" description:"Polling schedule of the binding, overriding config.advanced.poll. Same format."`
	Template string   `yaml:"template,omitempty" description:"Query template the connector renders and runs instead of its own, to capture the result of a custom query."`
}

// ResourcePath returns the binding name, satisfying capture.ResourcePather.
func (resource Resource) ResourcePath() []string {
	return []string{resource.Name}
}
