package dekaf

import "github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"

// Variant is the kind of consumer the Dekaf connector is tailored to.
type Variant string

// DeletionMode is how source deletions reach Kafka consumers.
type DeletionMode string

const (
	VariantGeneric      Variant = "generic"
	VariantMaterialize  Variant = "materialize"
	VariantTinybird     Variant = "tinybird"
	VariantClickHouse   Variant = "clickhouse"
	VariantRisingWave   Variant = "risingwave"
	VariantSingleStore  Variant = "singlestore"
	VariantStarTree     Variant = "startree"
	VariantImplyPolaris Variant = "imply-polaris"
	VariantBytewax      Variant = "bytewax"
)

// Variants lists every supported variant, useful for schema validators.
var Variants = []Variant{
	VariantGeneric,
	VariantMaterialize,
	VariantTinybird,
	VariantClickHouse,
	VariantRisingWave,
	VariantSingleStore,
	VariantStarTree,
	VariantImplyPolaris,
	VariantBytewax,
}

const (
	DeletionModeKafka DeletionMode = "kafka"
	DeletionModeCDC   DeletionMode = "cdc"
)

// DeletionModes lists every supported deletion mode, useful for schema validators.
var DeletionModes = []DeletionMode{
	DeletionModeKafka,
	DeletionModeCDC,
}

// Config is the Dekaf connector configuration.
type Config struct {
	Token            string       `yaml:"token" description:"Password Kafka consumers authenticate with, using the full materialization name as username. Write-only."`
	StrictTopicNames bool         `yaml:"strict_topic_names" description:"Exposes topic names in a strictly Kafka-compliant format, for consumers that require it. The dashboard sets false."`
	Deletions        DeletionMode `yaml:"deletions" enum:"kafka,cdc" description:"How deletions reach consumers: 'kafka' emits Kafka records with null values, 'cdc' emits the deletion document with an _is_deleted field, useful for hard deletes. The dashboard sets 'kafka'."`
}

// Resource identifies the target Dekaf topic.
type Resource struct {
	TopicName string `yaml:"topic_name" description:"Kafka topic the collection is exposed as, through the Kafka metadata and discovery APIs."`
}

// Endpoint holds the Dekaf variant and its configuration.
type Endpoint struct {
	Variant Variant `yaml:"variant" enum:"generic,materialize,tinybird,clickhouse,risingwave,singlestore,startree,imply-polaris,bytewax"`
	Config  Config  `yaml:"config"`
}

// EndpointSpec wraps the endpoint under the "dekaf" key, as in endpoint.dekaf.
type EndpointSpec struct {
	Dekaf Endpoint `yaml:"dekaf"`
}

// Binding maps a source collection to its Dekaf topic.
type Binding = materialization.Binding[Resource]

// Spec is a Dekaf materialization entry.
type Spec = materialization.Spec[EndpointSpec, Resource]

// ConnectorName returns the catalog name suffix of the variant, e.g. dekaf-generic.
func ConnectorName(variant Variant) string {
	return "dekaf-" + string(variant)
}
