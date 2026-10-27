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
	Token            string       `yaml:"token"`
	StrictTopicNames bool         `yaml:"strict_topic_names"`
	Deletions        DeletionMode `yaml:"deletions" enum:"kafka,cdc"`
}

// Resource identifies the target Dekaf topic.
type Resource struct {
	TopicName string `yaml:"topic_name"`
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
