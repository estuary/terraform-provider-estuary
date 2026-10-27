package dynamodb

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunMaterializationSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, auth.AWSAuthTypes, materialization.OnIncompatibleSchemaChanges, materialization.TargetNamingStrategies)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	expectations := map[string]auth.AWSCredentials{
		"acmeCo/dynamodb/access-key": {
			AuthType:           auth.AWSAuthTypeAccessKey,
			AWSAccessKeyID:     "example-aws-access-key-id",
			AWSSecretAccessKey: "example-aws-secret-access-key",
		},
		"acmeCo/dynamodb/iam": {
			AuthType:   auth.AWSAuthTypeIAM,
			AWSRoleARN: "arn:aws:iam::123456789012:role/estuary-dynamodb",
			AWSRegion:  "us-east-1",
		},
	}

	if len(specs) != len(expectations) {
		test.Fatalf("got %d materializations, want %d", len(specs), len(expectations))
	}

	for name, expected := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			if !found {
				test.Fatal("materialization missing from sample")
			}

			var config Config = spec.Endpoint.Connector.Config

			if config.Region != "us-east-1" {
				test.Errorf("region = %q, want us-east-1", config.Region)
			}

			if config.Credentials != expected {
				test.Errorf("credentials = %+v, want %+v", config.Credentials, expected)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/analytics/materialize-dynamodb"]

	var config Config = spec.Endpoint.Connector.Config

	expectedAdvanced := Advanced{
		Endpoint:     "http://localhost:8000",
		FeatureFlags: "retain_existing_data_on_backfill",
	}

	if config.Region != "eu-west-1" || config.Credentials.AuthType != auth.AWSAuthTypeIAM {
		test.Errorf("region = %q, credentials = %+v, want eu-west-1 with AWSIAM", config.Region, config.Credentials)
	}

	if config.Advanced == nil || *config.Advanced != expectedAdvanced {
		test.Errorf("advanced = %+v, want %+v", config.Advanced, expectedAdvanced)
	}

	if len(spec.Bindings) != 2 || spec.Bindings[1].Resource.Table != "customers.v2" {
		test.Errorf("bindings = %+v, want orders and customers.v2", spec.Bindings)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var invalidTableCharacters *regexp.Regexp = regexp.MustCompile(`[^.\-_0-9a-zA-Z]`)

	var seenAuthTypes map[auth.AWSAuthType]bool = map[auth.AWSAuthType]bool{}

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, file)

		for name, spec := range specs {
			var credentials auth.AWSCredentials = spec.Endpoint.Connector.Config.Credentials

			if !slices.Contains(auth.AWSAuthTypes, credentials.AuthType) {
				test.Errorf("%s: auth_type %q is not in AWSAuthTypes", name, credentials.AuthType)
			}

			if credentials.AuthType == auth.AWSAuthTypeAccessKey && (credentials.AWSAccessKeyID == "" || credentials.AWSSecretAccessKey == "") {
				test.Errorf("%s: AWSAccessKey credentials need aws_access_key_id and aws_secret_access_key", name)
			}

			if credentials.AuthType == auth.AWSAuthTypeIAM && (credentials.AWSRoleARN == "" || credentials.AWSRegion == "") {
				test.Errorf("%s: AWSIAM credentials need aws_role_arn and aws_region", name)
			}

			for _, binding := range spec.Bindings {
				if len(invalidTableCharacters.ReplaceAllString(binding.Resource.Table, "_")) < 3 {
					test.Errorf("%s: table %q needs at least 3 alphanumeric, dash, dot or underscore characters", name, binding.Resource.Table)
				}
			}

			seenAuthTypes[credentials.AuthType] = true
		}
	}

	for _, authType := range auth.AWSAuthTypes {
		if !seenAuthTypes[authType] {
			test.Errorf("auth_type %q has no sample", authType)
		}
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]struct {
		key     string
		content string
	}{
		"awsAccessKeyId is the hidden legacy access key": {
			key: "awsAccessKeyId",
			content: `
				materializations:
					acmeCo/dynamodb/legacy:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-dynamodb:v1
								config:
									awsAccessKeyId: example-aws-access-key-id
									awsSecretAccessKey: example-aws-secret-access-key
									region: us-east-1
						bindings: []
			`,
		},
		"rcuAllocation is a DynamoDB capture resource field": {
			key: "rcuAllocation",
			content: `
				materializations:
					acmeCo/dynamodb/rcu:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-dynamodb:v1
								config:
									region: us-east-1
									credentials:
										auth_type: AWSIAM
										aws_role_arn: arn:aws:iam::123456789012:role/estuary-dynamodb
										aws_region: us-east-1
						bindings:
							- resource:
									table: orders
									rcuAllocation: 1000
								source: acmeCo/orders
			`,
		},
	}

	for name, document := range documents {
		test.Run(name, func(test *testing.T) {
			_, err := spectest.ParseMaterializations[Spec]([]byte(document.content))

			if err == nil || !strings.Contains(err.Error(), "field "+document.key+" not found") {
				test.Fatalf("error = %v, want strict decoding to reject the %s key", err, document.key)
			}
		})
	}
}
