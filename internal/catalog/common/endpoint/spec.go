package endpoint

// Connector is the connector image and its config.
type Connector[C any] struct {
	Image  string `yaml:"image"`
	Config C      `yaml:"config"`
}

// ConnectorSpec wraps the connector under the "connector" key, as in endpoint.connector.
type ConnectorSpec[C any] struct {
	Connector Connector[C] `yaml:"connector"`
}

// SSHForwarding identifies the SSH server tunneling the connection to the endpoint.
type SSHForwarding struct {
	SSHEndpoint string `yaml:"sshEndpoint" credentials:"password" description:"Endpoint of the SSH server, in the form ssh://user@hostname[:port]. The plan rejects a password in it, such as user:password@, as state and logs would hold it in plain text: authenticate with private_key. A password found in the live value shows as ***."`
	PrivateKey  string `yaml:"privateKey" description:"Private key used to connect to the SSH server. Write-only."`
}

// NetworkTunnel connects to the endpoint through an SSH server acting as a bastion host.
type NetworkTunnel struct {
	SSHForwarding *SSHForwarding `yaml:"sshForwarding,omitempty" description:"SSH server tunneling the connection to the endpoint."`
}
