package dekaf

import "github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"

// Variant identifies the kind of consumer to which the Dekaf materialization connector is tailored to.
type Variant string

// DeletionMode controls how source deletions are surfaced to Kafka consumers.
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

// Config carries the Dekaf materialization connector configurations, including
// Authentication configs through the Token property and deletion and topic naming
// strategy.
type Config struct {
	Token            string       `yaml:"token"`
	StrictTopicNames bool         `yaml:"strict_topic_names"`
	Deletions        DeletionMode `yaml:"deletions"`
}

// Resource identifies the base Dekaf resource characteristics, including the target topic name to be generated.
type Resource struct {
	TopicName string `yaml:"topic_name"`
}

// Endpoint carries the Dekaf connector endpoint configurations that
// will reflect in the connector specs in Estuary platform
type Endpoint struct {
	Variant Variant `yaml:"variant"`
	Config  Config  `yaml:"config"`
}

// EndpointSpec wraps the endpoint under the "dekaf" key, as in endpoint.dekaf.
type EndpointSpec struct {
	Dekaf Endpoint `yaml:"dekaf"`
}

// ConnectorName returns the name ending the catalog name of every task of the variant, e.g. dekaf-generic.
func ConnectorName(variant Variant) string {
	return "dekaf-" + string(variant)
}

// Binding maps a source collection to the Dekaf topic exposing it.
type Binding = materialization.Binding[Resource]

// Spec is a full Dekaf materialization entry, keyed by its catalog name in a catalog spec.
type Spec = materialization.Spec[EndpointSpec, Resource]
