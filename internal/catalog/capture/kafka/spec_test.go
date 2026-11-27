package kafka

import (
	"reflect"
	"slices"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/kafka"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunCaptureSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, Resource](test)
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, AuthTypes, SchemaRegistryTypes, kafka.SASLMechanisms, kafka.TLSSettings)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/variants.yaml")
	var jsonOnly bool = true

	expectations := map[string]Config{
		"acmeCo/kafka/sasl-with-registry": {
			Connection:  kafka.Connection{BootstrapServers: "broker-1.example.com:9092,broker-2.example.com:9092", TLS: kafka.TLSSystemCertificates},
			Credentials: Credentials{AuthType: kafka.AuthTypeUserPassword, SASL: kafka.SASL{Mechanism: kafka.SASLMechanismScramSHA512, Username: "flow_capture", Password: "<secret>"}},
			SchemaRegistry: SchemaRegistry{
				Type:     SchemaRegistryTypeConfluent,
				Endpoint: "https://psrc-12345.us-east-2.aws.confluent.cloud",
				Username: "<registry-key>",
				Password: "<registry-secret>",
			},
		},
		"acmeCo/kafka/msk-json-only": {
			Connection:     kafka.Connection{BootstrapServers: "b-1.acme.abc123.c2.kafka.us-east-1.amazonaws.com:9098"},
			Credentials:    Credentials{AuthType: AuthTypeAWS, AWSAccessKeyID: "AKIAIOSFODNN7EXAMPLE", AWSSecretAccessKey: "<secret>", Region: "us-east-1"},
			SchemaRegistry: SchemaRegistry{Type: SchemaRegistryTypeNone, EnableJSONOnly: &jsonOnly},
		},
		"acmeCo/kafka/sasl-plain": {
			Connection:     kafka.Connection{BootstrapServers: "broker-1.example.com:9092"},
			Credentials:    Credentials{AuthType: kafka.AuthTypeUserPassword, SASL: kafka.SASL{Mechanism: kafka.SASLMechanismPlain, Username: "flow_capture", Password: "<secret>"}},
			SchemaRegistry: SchemaRegistry{Type: SchemaRegistryTypeNone, EnableJSONOnly: &jsonOnly},
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

			if config := spec.Endpoint.Connector.Config; !reflect.DeepEqual(config, expected) {
				test.Errorf("config = %+v, want %+v", config, expected)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/kafka/all-options"]

	if spec.AutoDiscover == nil || !spec.AutoDiscover.AddNewBindings || !spec.AutoDiscover.EvolveIncompatibleCollections {
		test.Errorf("autoDiscover = %+v, want both options enabled", spec.AutoDiscover)
	}

	expectedBindings := []Binding{
		{Resource: Resource{Topic: "orders"}, Target: "acmeCo/kafka/orders"},
		{Resource: Resource{Topic: "payments"}, Target: "acmeCo/kafka/payments", Backfill: 2},
		{Resource: Resource{Topic: "audit-log"}, Target: "acmeCo/kafka/audit-log", Disable: true},
	}

	if !reflect.DeepEqual(spec.Bindings, expectedBindings) {
		test.Errorf("bindings = %+v, want %+v", spec.Bindings, expectedBindings)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var authTypes *spectest.EnumSamples[kafka.AuthType] = spectest.NewEnumSamples(test, "auth_type", AuthTypes)
	var registryTypes *spectest.EnumSamples[SchemaRegistryType] = spectest.NewEnumSamples(test, "schema_registry_type", SchemaRegistryTypes)
	var mechanisms *spectest.EnumSamples[kafka.SASLMechanism] = spectest.NewEnumSamples(test, "mechanism", kafka.SASLMechanisms)
	var tls map[bool]bool = map[bool]bool{}

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeCaptures[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			var credentials Credentials = config.Credentials

			var registry SchemaRegistry = config.SchemaRegistry

			authTypes.Add(name, credentials.AuthType)
			registryTypes.Add(name, registry.Type)

			tls[config.TLS != ""] = true

			var sasl bool = credentials.Mechanism != "" && credentials.Username != "" && credentials.Password != ""
			var aws bool = credentials.AWSAccessKeyID != "" && credentials.AWSSecretAccessKey != "" && credentials.Region != ""

			if credentials.AuthType == kafka.AuthTypeUserPassword {
				mechanisms.Add(name, credentials.Mechanism)
			}

			if (credentials.AuthType == kafka.AuthTypeUserPassword) != sasl || (credentials.AuthType == AuthTypeAWS) != aws {
				test.Errorf("%s: %s credentials %+v must set every field of their auth type and no other", name, credentials.AuthType, credentials)
			}

			var confluent bool = registry.Endpoint != "" && registry.Username != "" && registry.Password != ""

			if (registry.Type == SchemaRegistryTypeConfluent) != confluent || (registry.Type == SchemaRegistryTypeNone) != (registry.EnableJSONOnly != nil && *registry.EnableJSONOnly) {
				test.Errorf("%s: schema_registry %+v must set the fields of its type and no other", name, registry)
			}
		}
	}

	authTypes.AssertAllSampled()
	registryTypes.AssertAllSampled()
	mechanisms.AssertAllSampled()

	if !tls[true] || !tls[false] {
		test.Errorf("captures sampled with tls = %v, want one with and one without", tls)
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"message_format is a materialization option": {
			Key: "message_format",
			Content: `
				captures:
					acmeCo/kafka/message-format:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-kafka:v1
								config:
									bootstrap_servers: broker-1.example.com:9092
									message_format: JSON
									credentials:
										auth_type: user_password
									schema_registry:
										schema_registry_type: no_schema_registry
						bindings: []
			`,
		},
		"partition isn't part of the resource": {
			Key: "partition",
			Content: `
				captures:
					acmeCo/kafka/partition:
						endpoint:
							connector:
								image: ghcr.io/estuary/source-kafka:v1
								config:
									bootstrap_servers: broker-1.example.com:9092
									credentials:
										auth_type: user_password
									schema_registry:
										schema_registry_type: no_schema_registry
						bindings:
							- resource:
									topic: orders
									partition: 0
								target: acmeCo/kafka/orders
			`,
		},
	}

	spectest.AssertUnknownCaptureKeys[Spec](test, documents)
}

func TestResourcePath(test *testing.T) {
	if path := (Resource{Topic: "orders"}).ResourcePath(); !slices.Equal(path, []string{"orders"}) {
		test.Errorf("resource path = %q, want [orders]", path)
	}
}
