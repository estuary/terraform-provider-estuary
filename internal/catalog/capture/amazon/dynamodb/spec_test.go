package dynamodb

import (
	"testing"
	"time"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/auth"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunCaptureSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, Resource](test)
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, auth.AWSAuthTypes)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		region      string
		credentials auth.AWSCredentials
	}{
		"acmeCo/dynamodb/access-key": {
			region: "us-east-1",
			credentials: auth.AWSCredentials{
				AuthType:           auth.AWSAuthTypeAccessKey,
				AWSAccessKeyID:     "example-aws-access-key-id",
				AWSSecretAccessKey: "example-aws-secret-access-key",
			},
		},
		"acmeCo/dynamodb/iam": {
			region: "us-east-1",
			credentials: auth.AWSCredentials{
				AuthType:   auth.AWSAuthTypeIAM,
				AWSRoleARN: "arn:aws:iam::123456789012:role/estuary-dynamodb",
				AWSRegion:  "us-east-1",
			},
		},
	}

	if len(specs) != len(expectations) {
		test.Fatalf("got %d captures, want %d", len(specs), len(expectations))
	}

	for name, expected := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			if !found {
				test.Fatal("capture missing from sample")
			}

			var config Config = spec.Endpoint.Connector.Config

			if config.Region != expected.region {
				test.Errorf("region = %q, want %q", config.Region, expected.region)
			}

			if config.Credentials != expected.credentials {
				test.Errorf("credentials = %+v, want %+v", config.Credentials, expected.credentials)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/dynamodb/all-options"]

	var config Config = spec.Endpoint.Connector.Config

	expectedAdvanced := Advanced{
		BackfillSegments: 50,
		ScanLimit:        500,
		IdleBackoffMax:   "2m",
		Endpoint:         "http://localhost:8000",
	}

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	if config.Region != "eu-west-1" || config.Credentials.AuthType != auth.AWSAuthTypeIAM {
		test.Errorf("region = %q, credentials = %+v, want eu-west-1 with AWSIAM", config.Region, config.Credentials)
	}

	if config.Advanced == nil || *config.Advanced != expectedAdvanced {
		test.Errorf("advanced = %+v, want %+v", config.Advanced, expectedAdvanced)
	}
}

func TestSampleBindings(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var bindings []Binding = specs["acmeCo/dynamodb/all-options"].Bindings

	expectations := []struct {
		table         string
		rcuAllocation int
		backfill      int
		disable       bool
	}{
		{"orders", 0, 0, false},
		{"customers", 1000, 1, false},
		{"sessions", 0, 0, true},
	}

	if len(bindings) != len(expectations) {
		test.Fatalf("got %d bindings, want %d", len(bindings), len(expectations))
	}

	for index, expected := range expectations {
		var binding Binding = bindings[index]

		if binding.Resource.Table != expected.table || binding.Resource.RCUAllocation != expected.rcuAllocation {
			test.Errorf("binding %d resource = %+v, want table %q rcuAllocation %d", index, binding.Resource, expected.table, expected.rcuAllocation)
		}

		if binding.Backfill != expected.backfill || binding.Disable != expected.disable {
			test.Errorf("binding %d backfill = %d, disable = %v, want %d and %v", index, binding.Backfill, binding.Disable, expected.backfill, expected.disable)
		}
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var authTypes *spectest.EnumSamples[auth.AWSAuthType] = spectest.NewEnumSamples(test, "auth_type", auth.AWSAuthTypes)

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			authTypes.Add(name, config.Credentials.AuthType)

			spectest.AssertAWSCredentials(test, name, config.Credentials)

			if config.Advanced != nil && config.Advanced.IdleBackoffMax != "" {
				duration, err := time.ParseDuration(config.Advanced.IdleBackoffMax)

				if err != nil || duration < time.Second || duration > 10*time.Minute {
					test.Errorf("%s: idleBackoffMax %q must be a duration between 1s and 10m", name, config.Advanced.IdleBackoffMax)
				}
			}
		}
	}

	authTypes.AssertAllSampled()
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"address is a database capture field": {
			Key: "address",
			Content: `
				captures:
					acmeCo/dynamodb/address:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-dynamodb:v1
								config:
									address: dynamodb.us-east-1.amazonaws.com
									region: us-east-1
						bindings: []
			`,
		},
		"awsAccessKeyId is the hidden legacy access key": {
			Key: "awsAccessKeyId",
			Content: `
				captures:
					acmeCo/dynamodb/legacy:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-dynamodb:v1
								config:
									awsAccessKeyId: example-aws-access-key-id
									awsSecretAccessKey: example-aws-secret-access-key
									region: us-west-2
						bindings: []
			`,
		},
		"stream is a SQL capture resource field": {
			Key: "stream",
			Content: `
				captures:
					acmeCo/dynamodb/stream:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-dynamodb:v1
								config:
									region: us-east-1
						bindings:
							- resource:
									table: orders
									stream: orders
								target: acmeCo/dynamodb/orders
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}
