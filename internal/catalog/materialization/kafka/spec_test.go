package kafka

import (
	"reflect"
	"slices"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/kafka"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunMaterializationSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, Resource](test)
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, AuthTypes, MessageFormats, Compressions, kafka.SASLMechanisms, kafka.TLSSettings, materialization.OnIncompatibleSchemaChanges, materialization.TargetNamingStrategies)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	expectations := map[string]Config{
		"acmeCo/kafka/avro-with-registry": {
			Connection:     kafka.Connection{BootstrapServers: "broker-1.example.com:9092,broker-2.example.com:9092", TLS: kafka.TLSSystemCertificates},
			Credentials:    Credentials{AuthType: kafka.AuthTypeUserPassword, SASL: kafka.SASL{Mechanism: kafka.SASLMechanismScramSHA512, Username: "flow_materialize", Password: "<secret>"}},
			MessageFormat:  MessageFormatAvro,
			SchemaRegistry: &SchemaRegistry{Endpoint: "https://psrc-12345.us-east-2.aws.confluent.cloud", Username: "<registry-key>", Password: "<registry-secret>"},
		},
		"acmeCo/kafka/json": {
			Connection:    kafka.Connection{BootstrapServers: "broker-1.example.com:9092"},
			Credentials:   Credentials{AuthType: kafka.AuthTypeUserPassword, SASL: kafka.SASL{Mechanism: kafka.SASLMechanismPlain, Username: "flow_materialize", Password: "<secret>"}},
			MessageFormat: MessageFormatJSON,
		},
		"acmeCo/kafka/scram-sha-256": {
			Connection:    kafka.Connection{BootstrapServers: "broker-1.example.com:9092"},
			Credentials:   Credentials{AuthType: kafka.AuthTypeUserPassword, SASL: kafka.SASL{Mechanism: kafka.SASLMechanismScramSHA256, Username: "flow_materialize", Password: "<secret>"}},
			MessageFormat: MessageFormatJSON,
			Compression:   CompressionNone,
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

			if config := spec.Endpoint.Connector.Config; !reflect.DeepEqual(config, expected) {
				test.Errorf("config = %+v, want %+v", config, expected)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/analytics/materialize-kafka"]

	var config Config = spec.Endpoint.Connector.Config

	if config.Compression != CompressionZstd || config.TopicPartitions != 12 || config.TopicReplicationFactor != 2 {
		test.Errorf("compression = %q, topic_partitions = %d, topic_replication_factor = %d", config.Compression, config.TopicPartitions, config.TopicReplicationFactor)
	}

	if config.AvroLogicalTypes == nil || *config.AvroLogicalTypes {
		test.Errorf("avro_logical_types = %v, want an explicit false", config.AvroLogicalTypes)
	}

	spectest.AssertMaterializationResources(test, spec.Bindings, []Resource{{Topic: "orders"}, {Topic: "customers"}})

	if spec.Bindings[1].Fields == nil || !slices.Equal(spec.Bindings[1].Fields.Exclude, []string{"payload"}) {
		test.Errorf("customers fields = %+v, want exclude [payload]", spec.Bindings[1].Fields)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var authTypes *spectest.EnumSamples[kafka.AuthType] = spectest.NewEnumSamples(test, "auth_type", AuthTypes)
	var formats *spectest.EnumSamples[MessageFormat] = spectest.NewEnumSamples(test, "message_format", MessageFormats)
	var mechanisms *spectest.EnumSamples[kafka.SASLMechanism] = spectest.NewEnumSamples(test, "mechanism", kafka.SASLMechanisms)
	var tls map[bool]bool = map[bool]bool{}

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			authTypes.Add(name, config.Credentials.AuthType)
			formats.Add(name, config.MessageFormat)
			mechanisms.Add(name, config.Credentials.Mechanism)

			tls[config.TLS != ""] = true

			if config.Credentials.Username == "" || config.Credentials.Password == "" {
				test.Errorf("%s: user_password credentials need username and password", name)
			}

			if (config.MessageFormat == MessageFormatAvro) != (config.SchemaRegistry != nil) {
				test.Errorf("%s: %s messages with schema_registry = %+v, want a schema registry with Avro only", name, config.MessageFormat, config.SchemaRegistry)
			}
		}
	}

	authTypes.AssertAllSampled()
	formats.AssertAllSampled()
	mechanisms.AssertAllSampled()

	if !tls[true] || !tls[false] {
		test.Errorf("materializations sampled with tls = %v, want one with and one without", tls)
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"aws_access_key_id belongs to the capture's AWS credentials": {
			Key: "aws_access_key_id",
			Content: `
				materializations:
					acmeCo/kafka/msk:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-kafka:v1
								config:
									bootstrap_servers: broker-1.example.com:9092
									credentials:
										auth_type: user_password
										aws_access_key_id: AKIAIOSFODNN7EXAMPLE
									message_format: JSON
						bindings: []
			`,
		},
		"schema_registry_type belongs to the capture's schema registry": {
			Key: "schema_registry_type",
			Content: `
				materializations:
					acmeCo/kafka/registry-type:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-kafka:v1
								config:
									bootstrap_servers: broker-1.example.com:9092
									credentials:
										auth_type: user_password
									message_format: Avro
									schema_registry:
										schema_registry_type: confluent_schema_registry
										endpoint: https://psrc-12345.us-east-2.aws.confluent.cloud
										username: <registry-key>
										password: <registry-secret>
						bindings: []
			`,
		},
		"delta_updates isn't part of the resource": {
			Key: "delta_updates",
			Content: `
				materializations:
					acmeCo/kafka/delta-updates:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-kafka:v1
								config:
									bootstrap_servers: broker-1.example.com:9092
									credentials:
										auth_type: user_password
									message_format: JSON
						bindings:
							- resource:
									topic: orders
									delta_updates: true
								source: acmeCo/orders
			`,
		},
	}

	spectest.AssertUnknownMaterializationKeys[Spec](test, documents)
}
