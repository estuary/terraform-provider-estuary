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

// DatabaseCredentials is the database authentication of SQL connectors, with the fields of its AuthType.
type DatabaseCredentials struct {
	AuthType                        DatabaseAuthType `yaml:"auth_type" enum:"UserPassword,AWSIAM,GCPIAM,AzureIAM" description:"Authentication method: 'UserPassword', 'AWSIAM', 'GCPIAM' or 'AzureIAM'. Only the attributes of the selected method are used."`
	Password                        string           `yaml:"password,omitempty" description:"Password of the database user, with 'UserPassword'. Write-only."`
	AWSRegion                       string           `yaml:"aws_region,omitempty" description:"AWS region of the database (e.g., 'us-east-1'), with 'AWSIAM'."`
	AWSRoleARN                      string           `yaml:"aws_role_arn,omitempty" description:"ARN of the IAM role Estuary assumes to connect, with 'AWSIAM'. See https://docs.estuary.dev/guides/iam-auth/aws/."`
	GCPServiceAccountToImpersonate  string           `yaml:"gcp_service_account_to_impersonate,omitempty" description:"Email of the Google Cloud service account used for IAM authentication, with 'GCPIAM'."`
	GCPWorkloadIdentityPoolAudience string           `yaml:"gcp_workload_identity_pool_audience,omitempty" description:"Audience of the workload identity pool, with 'GCPIAM', in the form //iam.googleapis.com/projects/123456789012/locations/global/workloadIdentityPools/<pool>/providers/<provider>."`
	AzureClientID                   string           `yaml:"azure_client_id,omitempty" description:"Client ID of the Azure app registration used for Microsoft Entra authentication, with 'AzureIAM'."`
	AzureTenantID                   string           `yaml:"azure_tenant_id,omitempty" description:"Azure tenant ID of the app registration, with 'AzureIAM'."`
}

// AWSCredentials is the AWS authentication of AWS connectors, with the fields of its AuthType.
type AWSCredentials struct {
	AuthType           AWSAuthType `yaml:"auth_type" enum:"AWSAccessKey,AWSIAM" description:"Authentication method: 'AWSAccessKey' or 'AWSIAM'. Only the attributes of the selected method are used."`
	AWSAccessKeyID     string      `yaml:"aws_access_key_id,omitempty" description:"AWS access key ID of the IAM user the connector authenticates as, with 'AWSAccessKey'."`
	AWSSecretAccessKey string      `yaml:"aws_secret_access_key,omitempty" description:"AWS secret access key of the IAM user, with 'AWSAccessKey'. Write-only."`
	AWSRoleARN         string      `yaml:"aws_role_arn,omitempty" description:"ARN of the IAM role Estuary assumes to connect, with 'AWSIAM'. See https://docs.estuary.dev/guides/iam-auth/aws/."`
	AWSRegion          string      `yaml:"aws_region,omitempty" description:"AWS region to authenticate in (e.g., 'us-east-1'), with 'AWSIAM'."`
}
