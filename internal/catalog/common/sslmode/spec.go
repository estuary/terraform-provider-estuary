package sslmode

// Postgres controls the connection SSL behavior through the PostgreSQL 'sslmode' parameter.
type Postgres string

// MySQL controls whether connections use TLS and how strictly the server certificate is verified.
type MySQL string

const (
	PostgresDisable    Postgres = "disable"
	PostgresAllow      Postgres = "allow"
	PostgresPrefer     Postgres = "prefer"
	PostgresRequire    Postgres = "require"
	PostgresVerifyCA   Postgres = "verify-ca"
	PostgresVerifyFull Postgres = "verify-full"
)

// PostgresModes lists every supported PostgreSQL SSL mode, useful for schema validators.
var PostgresModes = []Postgres{
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
var MySQLModes = []MySQL{
	MySQLDisabled,
	MySQLPreferred,
	MySQLRequired,
	MySQLVerifyCA,
	MySQLVerifyIdentity,
}
