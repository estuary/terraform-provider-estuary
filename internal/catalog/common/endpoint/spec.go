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
	SSHEndpoint string `yaml:"sshEndpoint"`
	PrivateKey  string `yaml:"privateKey"`
}

// NetworkTunnel connects to the endpoint through an SSH server acting as a bastion host.
type NetworkTunnel struct {
	SSHForwarding *SSHForwarding `yaml:"sshForwarding,omitempty"`
}
