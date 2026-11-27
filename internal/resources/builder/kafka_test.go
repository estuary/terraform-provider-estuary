package builder

import (
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	sourcekafka "github.com/estuary/terraform-provider-estuary/internal/catalog/capture/kafka"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/kafka"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	materializekafka "github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/kafka"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var kafkaCaptureConnector Connector = Connector{
	TypeName:      "source_kafka",
	Image:         sourcekafka.Image,
	ConnectorName: sourcekafka.ConnectorName,
}

var kafkaMaterializationConnector Connector = Connector{
	TypeName:      "materialization_kafka",
	Image:         materializekafka.Image,
	ConnectorName: materializekafka.ConnectorName,
}

// kafkaCaptureLive is a Kafka capture as the dashboard publishes it: sealed SASL and schema registry passwords, the
// default TLS setting, and discovered bindings with their resource paths.
const kafkaCaptureLive string = `
	autoDiscover:
		addNewBindings: true
		evolveIncompatibleCollections: true
	endpoint:
		connector:
			image: ghcr.io/estuary/source-kafka:v1
			config:
				bootstrap_servers: broker-1.example.com:9092,broker-2.example.com:9092
				tls: system_certificates
				credentials:
					auth_type: user_password
					mechanism: SCRAM-SHA-512
					username: flow_capture
					password_sops: ENC[AES256_GCM,data:abc,type:str]
				schema_registry:
					schema_registry_type: confluent_schema_registry
					endpoint: https://psrc-12345.us-east-2.aws.confluent.cloud
					username: registry-key
					password_sops: ENC[AES256_GCM,data:def,type:str]
				sops:
					gcp_kms:
						- resource_id: projects/example/locations/us-central1/keyRings/example/cryptoKeys/example
							created_at: "2026-01-01T00:00:00Z"
							enc: <encrypted>
					lastmodified: "2026-01-01T00:00:00Z"
					mac: ENC[AES256_GCM,data:ghi,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				_meta:
					path: [orders]
				topic: orders
			target: acmeCo/kafka/orders
			backfill: 1
		- resource:
				_meta:
					path: [payments]
				topic: payments
			target: acmeCo/kafka/payments
	shards:
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

// kafkaCaptureJSONOnlyLive is a Kafka capture of an Amazon MSK cluster without a schema registry.
const kafkaCaptureJSONOnlyLive string = `
	endpoint:
		connector:
			image: ghcr.io/estuary/source-kafka:v1
			config:
				bootstrap_servers: b-1.acme.abc123.c2.kafka.us-east-1.amazonaws.com:9098
				tls: system_certificates
				credentials:
					auth_type: AWS
					aws_access_key_id: AKIAIOSFODNN7EXAMPLE
					aws_secret_access_key_sops: ENC[AES256_GCM,data:abc,type:str]
					region: us-east-1
				schema_registry:
					schema_registry_type: no_schema_registry
					enable_json_only: true
				sops:
					mac: ENC[AES256_GCM,data:ghi,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				_meta:
					path: [clicks]
				topic: clicks
			target: acmeCo/kafka/clicks
`

// kafkaMaterializationJSONLive is a Kafka materialization of JSON messages as the dashboard publishes it: a sealed
// SASL password, explicit defaults, and bindings with their resource paths.
const kafkaMaterializationJSONLive string = `
	endpoint:
		connector:
			image: ghcr.io/estuary/materialize-kafka:v1
			config:
				bootstrap_servers: broker-1.example.com:9092,broker-2.example.com:9092
				tls: system_certificates
				credentials:
					auth_type: user_password
					mechanism: SCRAM-SHA-512
					username: flow_materialize
					password_sops: ENC[AES256_GCM,data:abc,type:str]
				message_format: JSON
				compression: lz4
				avro_logical_types: true
				topic_partitions: 6
				topic_replication_factor: 3
				sops:
					gcp_kms:
						- resource_id: projects/example/locations/us-central1/keyRings/example/cryptoKeys/example
							created_at: "2026-01-01T00:00:00Z"
							enc: <encrypted>
					lastmodified: "2026-01-01T00:00:00Z"
					mac: ENC[AES256_GCM,data:ghi,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				_meta:
					path: [orders]
				topic: orders
			source: acmeCo/shop/orders
			fields:
				recommended: true
		- resource:
				_meta:
					path: [customers]
				topic: customers
			source: acmeCo/shop/customers
			fields:
				recommended: true
			backfill: 1
	shards:
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

// kafkaMaterializationAvroLive is a Kafka materialization of Avro messages, with the sealed password of its schema
// registry.
const kafkaMaterializationAvroLive string = `
	endpoint:
		connector:
			image: ghcr.io/estuary/materialize-kafka:v1
			config:
				bootstrap_servers: broker-1.example.com:9092
				credentials:
					auth_type: user_password
					mechanism: PLAIN
					username: flow_materialize
					password_sops: ENC[AES256_GCM,data:abc,type:str]
				message_format: Avro
				schema_registry:
					endpoint: https://psrc-12345.us-east-2.aws.confluent.cloud
					username: registry-key
					password_sops: ENC[AES256_GCM,data:def,type:str]
				avro_logical_types: false
				sops:
					mac: ENC[AES256_GCM,data:ghi,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				_meta:
					path: [orders]
				topic: orders
			source: acmeCo/shop/orders
			fields:
				recommended: true
	expectPubId: "11:22:33:44:55:66:77:88"
`

func kafkaCaptureSpec(discovers bool, config sourcekafka.Config, topics ...string) CaptureSpec[sourcekafka.Config, sourcekafka.Resource] {
	var spec CaptureSpec[sourcekafka.Config, sourcekafka.Resource] = CaptureSpec[sourcekafka.Config, sourcekafka.Resource]{
		Endpoint: endpoint.ConnectorSpec[sourcekafka.Config]{Connector: endpoint.Connector[sourcekafka.Config]{Image: sourcekafka.Image, Config: config}},
	}

	if discovers {
		spec.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true, EvolveIncompatibleCollections: true}
	}

	for _, topic := range topics {
		spec.Bindings = append(spec.Bindings, capture.Binding[sourcekafka.Resource]{Resource: sourcekafka.Resource{Topic: topic}, Target: "acmeCo/kafka/" + topic})
	}

	return spec
}

func kafkaMaterializationSpec(config materializekafka.Config, topics ...string) MaterializationSpec[materializekafka.Config, materializekafka.Resource] {
	var spec MaterializationSpec[materializekafka.Config, materializekafka.Resource] = MaterializationSpec[materializekafka.Config, materializekafka.Resource]{
		Endpoint: endpoint.ConnectorSpec[materializekafka.Config]{Connector: endpoint.Connector[materializekafka.Config]{Image: materializekafka.Image, Config: config}},
	}

	for _, topic := range topics {
		spec.Bindings = append(spec.Bindings, materialization.Binding[materializekafka.Resource]{
			Resource: materializekafka.Resource{Topic: topic},
			Source:   materialization.BindingSource{Name: "acmeCo/shop/" + topic},
		})
	}

	return spec
}

func TestKafkaSamplesRoundTrip(test *testing.T) {
	assertCaptureSamplesRoundTrip[sourcekafka.Config, sourcekafka.Resource](test, kafkaCaptureConnector, "../../catalog/capture/kafka/testdata")
	assertMaterializationSamplesRoundTrip[materializekafka.Config, materializekafka.Resource](test, kafkaMaterializationConnector, "../../catalog/materialization/kafka/testdata")
}

func TestKafkaCaptureImportPlansNoChanges(test *testing.T) {
	var jsonOnly bool = true

	var config sourcekafka.Config = sourcekafka.Config{
		Connection:  kafka.Connection{BootstrapServers: "broker-1.example.com:9092,broker-2.example.com:9092", TLS: kafka.TLSSystemCertificates},
		Credentials: sourcekafka.Credentials{AuthType: kafka.AuthTypeUserPassword, SASL: kafka.SASL{Mechanism: kafka.SASLMechanismScramSHA512, Username: "flow_capture"}},
		SchemaRegistry: sourcekafka.SchemaRegistry{
			Type:     sourcekafka.SchemaRegistryTypeConfluent,
			Endpoint: "https://psrc-12345.us-east-2.aws.confluent.cloud",
			Username: "registry-key",
		},
	}

	entry := newConnectorCapture[sourcekafka.Config, sourcekafka.Resource](test, kafkaCaptureConnector, kafkaCaptureLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/events/source-kafka", false)

	entry.assertPlansNoChanges(test, "acmeCo/events", kafkaCaptureSpec(true, config), state)
	entry.assertAppliesWithoutPublishing(test, "acmeCo/events", kafkaCaptureSpec(true, config, "orders", "payments"), state)

	var mskConfig sourcekafka.Config = sourcekafka.Config{
		Connection:     kafka.Connection{BootstrapServers: "b-1.acme.abc123.c2.kafka.us-east-1.amazonaws.com:9098", TLS: kafka.TLSSystemCertificates},
		Credentials:    sourcekafka.Credentials{AuthType: sourcekafka.AuthTypeAWS, AWSAccessKeyID: "AKIAIOSFODNN7EXAMPLE", Region: "us-east-1"},
		SchemaRegistry: sourcekafka.SchemaRegistry{Type: sourcekafka.SchemaRegistryTypeNone, EnableJSONOnly: &jsonOnly},
	}

	msk := newConnectorCapture[sourcekafka.Config, sourcekafka.Resource](test, kafkaCaptureConnector, kafkaCaptureJSONOnlyLive)

	state = msk.importState(test, "acmeCo/events/source-kafka", false)

	msk.assertPlansNoChanges(test, "acmeCo/events", kafkaCaptureSpec(false, mskConfig), state)
	msk.assertAppliesWithoutPublishing(test, "acmeCo/events", kafkaCaptureSpec(false, mskConfig, "clicks"), state)
}

func TestKafkaMaterializationImportPlansNoChanges(test *testing.T) {
	var logicalTypes bool = true

	var config materializekafka.Config = materializekafka.Config{
		Connection:             kafka.Connection{BootstrapServers: "broker-1.example.com:9092,broker-2.example.com:9092", TLS: kafka.TLSSystemCertificates},
		Credentials:            materializekafka.Credentials{AuthType: kafka.AuthTypeUserPassword, SASL: kafka.SASL{Mechanism: kafka.SASLMechanismScramSHA512, Username: "flow_materialize"}},
		MessageFormat:          materializekafka.MessageFormatJSON,
		Compression:            materializekafka.CompressionLZ4,
		AvroLogicalTypes:       &logicalTypes,
		TopicPartitions:        6,
		TopicReplicationFactor: 3,
	}

	entry := newConnectorMaterialization[materializekafka.Config, materializekafka.Resource](test, kafkaMaterializationConnector, kafkaMaterializationJSONLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/events/materialize-kafka", false)

	entry.assertPlansNoChanges(test, "acmeCo/events", kafkaMaterializationSpec(config), state)
	entry.assertAppliesWithoutPublishing(test, "acmeCo/events", kafkaMaterializationSpec(config, "orders", "customers"), state)
}

// The schema registry password is a required attribute, so a materialization of Avro messages can't leave its secrets
// out: the first plan after the import shows their hashes, and the apply publishes the endpoint with the configured
// secrets, leaving the bindings as they are live.
func TestKafkaMaterializationImportWithASchemaRegistryPlansOnlyItsSecrets(test *testing.T) {
	var logicalTypes bool = false

	var config materializekafka.Config = materializekafka.Config{
		Connection:       kafka.Connection{BootstrapServers: "broker-1.example.com:9092"},
		Credentials:      materializekafka.Credentials{AuthType: kafka.AuthTypeUserPassword, SASL: kafka.SASL{Mechanism: kafka.SASLMechanismPlain, Username: "flow_materialize", Password: "<secret>"}},
		MessageFormat:    materializekafka.MessageFormatAvro,
		SchemaRegistry:   &materializekafka.SchemaRegistry{Endpoint: "https://psrc-12345.us-east-2.aws.confluent.cloud", Username: "registry-key", Password: "<registry-secret>"},
		AvroLogicalTypes: &logicalTypes,
	}

	entry := newConnectorMaterialization[materializekafka.Config, materializekafka.Resource](test, kafkaMaterializationConnector, kafkaMaterializationAvroLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/events/materialize-kafka", false)

	entry.assertPlansOnlySecretHashes(test, "acmeCo/events", kafkaMaterializationSpec(config), state,
		configAttributePath("credentials", "password"),
		configAttributePath("schema_registry", "password"),
	)

	var live []any = catalog.Bindings(entry.backend.live)

	published, _ := entry.apply(test, "acmeCo/events", kafkaMaterializationSpec(config, "orders"), state)

	if !reflect.DeepEqual(catalog.Bindings(published), live) {
		test.Errorf("published bindings = %v, want the live ones, %v", catalog.Bindings(published), live)
	}

	var want map[string]any = map[string]any{
		"bootstrap_servers":  "broker-1.example.com:9092",
		"credentials":        map[string]any{"auth_type": "user_password", "mechanism": "PLAIN", "username": "flow_materialize", "password": "<secret>"},
		"message_format":     "Avro",
		"schema_registry":    map[string]any{"endpoint": "https://psrc-12345.us-east-2.aws.confluent.cloud", "username": "registry-key", "password": "<registry-secret>"},
		"avro_logical_types": false,
	}

	if config := utils.Lookup(published, catalog.EndpointKey, catalog.ConnectorKey, catalog.ConfigKey); !reflect.DeepEqual(config, want) {
		test.Errorf("published config = %v, want the live one with the configured secrets, %v", config, want)
	}
}

func TestKafkaSecretsAreWriteOnly(test *testing.T) {
	captureCodec, err := newCodec[captureDocument[sourcekafka.Config, sourcekafka.Resource]](kafkaCaptureConnector, captureType[sourcekafka.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	captureConfig, err := captureCodec.Value("events", kafkaCaptureSpec(false, sourcekafka.Config{
		Connection: kafka.Connection{BootstrapServers: "broker-1.example.com:9092"},
		Credentials: sourcekafka.Credentials{
			AuthType:           kafka.AuthTypeUserPassword,
			SASL:               kafka.SASL{Mechanism: kafka.SASLMechanismPlain, Username: "flow_capture", Password: "<secret>"},
			AWSAccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			AWSSecretAccessKey: "<aws-secret>",
		},
		SchemaRegistry: sourcekafka.SchemaRegistry{Type: sourcekafka.SchemaRegistryTypeConfluent, Endpoint: "https://registry.example.com", Username: "registry-key", Password: "<registry-secret>"},
	}))

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, captureCodec.secrets, captureConfig,
		configAttributePath("credentials", "password"),
		configAttributePath("credentials", "aws_secret_access_key"),
		configAttributePath("schema_registry", "password"),
	)

	materializationCodec, err := newCodec[materializationDocument[materializekafka.Config, materializekafka.Resource]](kafkaMaterializationConnector, materializationType[materializekafka.Config, materializekafka.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	materializationConfig, err := materializationCodec.Value("events", kafkaMaterializationSpec(materializekafka.Config{
		Connection:     kafka.Connection{BootstrapServers: "broker-1.example.com:9092"},
		Credentials:    materializekafka.Credentials{AuthType: kafka.AuthTypeUserPassword, SASL: kafka.SASL{Mechanism: kafka.SASLMechanismPlain, Username: "flow_materialize", Password: "<secret>"}},
		MessageFormat:  materializekafka.MessageFormatAvro,
		SchemaRegistry: &materializekafka.SchemaRegistry{Endpoint: "https://registry.example.com", Username: "registry-key", Password: "<registry-secret>"},
	}))

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, materializationCodec.secrets, materializationConfig,
		configAttributePath("credentials", "password"),
		configAttributePath("schema_registry", "password"),
	)
}
