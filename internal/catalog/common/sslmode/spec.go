package sslmode

// Postgres controls the connection SSL behavior through the PostgreSQL 'sslmode' parameter.
type Postgres string

// MySQL controls whether connections use TLS and how strictly the server certificate is verified.
type MySQL string

// MySQLTLS holds the MySQL TLS settings of capture and materialization endpoints, inlined in their advanced options.
// SSLServerCA is required by verify_ca, and by verify_identity on materializations.
type MySQLTLS struct {
	SSLMode       MySQL  `yaml:"sslmode,omitempty" enum:"disabled,preferred,required,verify_ca,verify_identity" description:"TLS of the connection: 'disabled' doesn't encrypt it, 'preferred' encrypts it when the server supports TLS, 'required' encrypts it without verifying the server certificate, 'verify_ca' also verifies the certificate against ssl_server_ca, and 'verify_identity' verifies the hostname as well. Captures default to 'required'."`
	SSLServerCA   string `yaml:"ssl_server_ca,omitempty" secret:"true" description:"PEM-encoded certificate authority the server certificate is verified against. Required with 'verify_ca', and with 'verify_identity' on materializations; captures otherwise trust public and managed database certificate authorities. Write-only."`
	SSLClientCert string `yaml:"ssl_client_cert,omitempty" secret:"true" description:"PEM-encoded client certificate presented to the server with ssl_client_key, for mutual TLS with 'verify_ca' and 'verify_identity'. Write-only."`
	SSLClientKey  string `yaml:"ssl_client_key,omitempty" description:"PEM-encoded private key of ssl_client_cert. Write-only."`
}

const (
	PostgresDisable    Postgres = "disable"
	PostgresAllow      Postgres = "allow"
	PostgresPrefer     Postgres = "prefer"
	PostgresRequire    Postgres = "require"
	PostgresVerifyCA   Postgres = "verify-ca"
	PostgresVerifyFull Postgres = "verify-full"
)

// PostgresModes lists every supported PostgreSQL SSL mode, useful for schema validators.
var PostgresModes []Postgres = []Postgres{
	PostgresDisable,
	PostgresAllow,
	PostgresPrefer,
	PostgresRequire,
	PostgresVerifyCA,
	PostgresVerifyFull,
}

const (
	MySQLDisabled       MySQL = "disabled"
	MySQLPreferred      MySQL = "preferred"
	MySQLRequired       MySQL = "required"
	MySQLVerifyCA       MySQL = "verify_ca"
	MySQLVerifyIdentity MySQL = "verify_identity"
)

// MySQLModes lists every supported MySQL SSL mode, useful for schema validators.
var MySQLModes []MySQL = []MySQL{
	MySQLDisabled,
	MySQLPreferred,
	MySQLRequired,
	MySQLVerifyCA,
	MySQLVerifyIdentity,
}
