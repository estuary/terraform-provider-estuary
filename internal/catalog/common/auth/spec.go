package auth

// DatabaseAuthType identifies how a SQL connector authenticates its database user.
type DatabaseAuthType string

// AWSAuthType identifies how an AWS connector authenticates with AWS.
type AWSAuthType string

const (
	DatabaseAuthTypeUserPassword DatabaseAuthType = "UserPassword"
	DatabaseAuthTypeAWSIAM       DatabaseAuthType = "AWSIAM"
	DatabaseAuthTypeGCPIAM       DatabaseAuthType = "GCPIAM"
	DatabaseAuthTypeAzureIAM     DatabaseAuthType = "AzureIAM"
)

// DatabaseAuthTypes lists every supported database auth type, useful for schema validators.
var DatabaseAuthTypes = []DatabaseAuthType{
	DatabaseAuthTypeUserPassword,
	DatabaseAuthTypeAWSIAM,
	DatabaseAuthTypeGCPIAM,
	DatabaseAuthTypeAzureIAM,
}

const (
	AWSAuthTypeAccessKey AWSAuthType = "AWSAccessKey"
	AWSAuthTypeIAM       AWSAuthType = "AWSIAM"
)

// AWSAuthTypes lists every supported AWS auth type, useful for schema validators.
var AWSAuthTypes = []AWSAuthType{
	AWSAuthTypeAccessKey,
	AWSAuthTypeIAM,
}

// DatabaseCredentials carries the database authentication of SQL captures and materializations,
// with the fields relevant to its AuthType.
type DatabaseCredentials struct {
	AuthType                        DatabaseAuthType `yaml:"auth_type"`
	Password                        string           `yaml:"password,omitempty"`
	AWSRegion                       string           `yaml:"aws_region,omitempty"`
	AWSRoleARN                      string           `yaml:"aws_role_arn,omitempty"`
	GCPServiceAccountToImpersonate  string           `yaml:"gcp_service_account_to_impersonate,omitempty"`
	GCPWorkloadIdentityPoolAudience string           `yaml:"gcp_workload_identity_pool_audience,omitempty"`
	AzureClientID                   string           `yaml:"azure_client_id,omitempty"`
	AzureTenantID                   string           `yaml:"azure_tenant_id,omitempty"`
}

// AWSCredentials carries the AWS authentication of AWS captures and materializations, with the
// fields relevant to its AuthType.
type AWSCredentials struct {
	AuthType           AWSAuthType `yaml:"auth_type"`
	AWSAccessKeyID     string      `yaml:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string      `yaml:"aws_secret_access_key,omitempty"`
	AWSRoleARN         string      `yaml:"aws_role_arn,omitempty"`
	AWSRegion          string      `yaml:"aws_region,omitempty"`
}
