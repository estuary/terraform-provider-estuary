package spectest

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
)

// AssertAWSCredentials checks the credentials of the named sample carry the fields of their auth type.
func AssertAWSCredentials(test testing.TB, name string, credentials auth.AWSCredentials) {
	test.Helper()

	if credentials.AuthType == auth.AWSAuthTypeAccessKey && (credentials.AWSAccessKeyID == "" || credentials.AWSSecretAccessKey == "") {
		test.Errorf("%s: AWSAccessKey credentials need aws_access_key_id and aws_secret_access_key", name)
	}

	if credentials.AuthType == auth.AWSAuthTypeIAM && (credentials.AWSRoleARN == "" || credentials.AWSRegion == "") {
		test.Errorf("%s: AWSIAM credentials need aws_role_arn and aws_region", name)
	}
}

// AssertDatabaseCredentials checks sample credentials carry the placeholder fields of their auth type and no other.
func AssertDatabaseCredentials(test testing.TB, credentials auth.DatabaseCredentials) {
	test.Helper()

	switch credentials.AuthType {
	case auth.DatabaseAuthTypeUserPassword:
		if credentials.Password != "<secret>" || credentials.AWSRoleARN != "" {
			test.Errorf("password credentials = %+v, want password and no role", credentials)
		}
	case auth.DatabaseAuthTypeAWSIAM:
		if credentials.AWSRoleARN == "" || credentials.AWSRegion != "us-east-1" || credentials.Password != "" {
			test.Errorf("IAM credentials = %+v, want role and region and no password", credentials)
		}
	case auth.DatabaseAuthTypeGCPIAM:
		if credentials.GCPServiceAccountToImpersonate == "" || credentials.GCPWorkloadIdentityPoolAudience == "" || credentials.Password != "" {
			test.Errorf("GCP IAM credentials = %+v, want service account and audience and no password", credentials)
		}
	case auth.DatabaseAuthTypeAzureIAM:
		if credentials.AzureClientID == "" || credentials.AzureTenantID == "" || credentials.Password != "" {
			test.Errorf("Azure IAM credentials = %+v, want client and tenant and no password", credentials)
		}
	}
}
