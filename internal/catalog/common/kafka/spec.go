// Package kafka holds the blocks the Kafka capture and materialization connectors share, so a cluster is configured the
// same way on both sides.
package kafka

// TLS is how the connectors find the TLS certificates they verify the brokers with.
type TLS string

// AuthType identifies how a Kafka connector authenticates with the cluster.
type AuthType string

// SASLMechanism is how a SASL user and password are exchanged with the brokers.
type SASLMechanism string

// TLSSystemCertificates verifies the brokers against the certificate authorities of the connector's system.
const TLSSystemCertificates TLS = "system_certificates"

// TLSSettings lists every supported TLS setting, useful for schema validators.
var TLSSettings = []TLS{
	TLSSystemCertificates,
}

// AuthTypeUserPassword authenticates with a SASL user and password, the method both connectors have.
const AuthTypeUserPassword AuthType = "user_password"

const (
	SASLMechanismPlain       SASLMechanism = "PLAIN"
	SASLMechanismScramSHA256 SASLMechanism = "SCRAM-SHA-256"
	SASLMechanismScramSHA512 SASLMechanism = "SCRAM-SHA-512"
)

// SASLMechanisms lists every supported SASL mechanism, useful for schema validators.
var SASLMechanisms = []SASLMechanism{
	SASLMechanismPlain,
	SASLMechanismScramSHA256,
	SASLMechanismScramSHA512,
}

// Connection is where the cluster is and how the brokers' TLS certificates are verified.
type Connection struct {
	BootstrapServers string `yaml:"bootstrap_servers" description:"Brokers the connector first connects to, as comma-separated host:port pairs (e.g., 'broker-1.example.com:9092,broker-2.example.com:9092'). They tell it about the other brokers of the cluster."`
	TLS              TLS    `yaml:"tls,omitempty" enum:"system_certificates" description:"How the connector finds the TLS certificates it verifies the brokers with: 'system_certificates', the default and only setting, uses the certificate authorities its system trusts."`
}

// SASL is the user and password of the 'user_password' credentials, inlined in the credentials of each connector.
type SASL struct {
	Mechanism SASLMechanism `yaml:"mechanism,omitempty" enum:"PLAIN,SCRAM-SHA-256,SCRAM-SHA-512" description:"SASL mechanism the user and password are exchanged with, with 'user_password': 'PLAIN', 'SCRAM-SHA-256' or 'SCRAM-SHA-512'."`
	Username  string        `yaml:"username,omitempty" description:"SASL user, with 'user_password'."`
	Password  string        `yaml:"password,omitempty" description:"Password of the SASL user, with 'user_password'. Write-only."`
}
