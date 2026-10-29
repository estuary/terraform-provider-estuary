package materializationdekaf

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/dekaf"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// variant fixes the Dekaf variant of a resource, which users never set.
type variant interface {
	variant() dekaf.Variant
}

type generic struct{}
type materialize struct{}
type tinybird struct{}
type clickHouse struct{}
type risingWave struct{}
type singleStore struct{}
type starTree struct{}
type implyPolaris struct{}
type bytewax struct{}

func (generic) variant() dekaf.Variant      { return dekaf.VariantGeneric }
func (materialize) variant() dekaf.Variant  { return dekaf.VariantMaterialize }
func (tinybird) variant() dekaf.Variant     { return dekaf.VariantTinybird }
func (clickHouse) variant() dekaf.Variant   { return dekaf.VariantClickHouse }
func (risingWave) variant() dekaf.Variant   { return dekaf.VariantRisingWave }
func (singleStore) variant() dekaf.Variant  { return dekaf.VariantSingleStore }
func (starTree) variant() dekaf.Variant     { return dekaf.VariantStarTree }
func (implyPolaris) variant() dekaf.Variant { return dekaf.VariantImplyPolaris }
func (bytewax) variant() dekaf.Variant      { return dekaf.VariantBytewax }

// document is the Terraform shape of a Dekaf materialization of variant V.
type document[V variant] struct {
	Name                        string `yaml:"name"`
	materialization.SpecOptions `yaml:",inline"`
	Config                      dekaf.Config                                     `yaml:"config" description:"Authentication and topic settings Kafka consumers see."`
	Bindings                    []builder.MaterializationBinding[dekaf.Resource] `yaml:"bindings" description:"Materialized collections, one topic per binding."`
	Backfill                    *builder.MaterializationBackfill                 `yaml:"backfill,omitempty" description:"Backfill control of the whole materialization."`
}

// Spec returns the Dekaf catalog name and spec, with the variant of V.
func (document document[V]) Spec() (string, dekaf.Spec) {
	var fixed V

	return document.Name, dekaf.Spec{
		Endpoint: dekaf.EndpointSpec{
			Dekaf: dekaf.Endpoint{
				Variant: fixed.variant(),
				Config:  document.Config,
			},
		},
		SpecOptions: document.SpecOptions,
		Bindings:    builder.MaterializationBindingsSpec(document.Bindings),
	}
}

// FromSpec returns the document of a Dekaf catalog name and spec.
func (document[V]) FromSpec(name string, spec dekaf.Spec) document[V] {
	return document[V]{
		Name:        name,
		SpecOptions: spec.SpecOptions,
		Config:      spec.Endpoint.Dekaf.Config,
		Bindings:    builder.NewMaterializationBindings(spec.Bindings),
	}
}

// newResource returns the factory of the resource of variant V, named after its dashboard tile.
func newResource[V variant](typeName string) func() resource.Resource {
	var fixed V

	return builder.NewMaterializationFromDocument[document[V]](builder.Connector{
		TypeName:      typeName,
		ConnectorName: dekaf.ConnectorName(fixed.variant()),
	})
}

// NewGeneric returns the estuary_materialization_dekaf_generic resource.
var NewGeneric func() resource.Resource = newResource[generic]("materialization_dekaf_generic")

// NewMaterialize returns the estuary_materialization_dekaf_materialize resource.
var NewMaterialize func() resource.Resource = newResource[materialize]("materialization_dekaf_materialize")

// NewTinybird returns the estuary_materialization_dekaf_tinybird resource.
var NewTinybird func() resource.Resource = newResource[tinybird]("materialization_dekaf_tinybird")

// NewClickHouse returns the estuary_materialization_dekaf_clickhouse resource.
var NewClickHouse func() resource.Resource = newResource[clickHouse]("materialization_dekaf_clickhouse")

// NewRisingWave returns the estuary_materialization_dekaf_risingwave resource.
var NewRisingWave func() resource.Resource = newResource[risingWave]("materialization_dekaf_risingwave")

// NewSingleStore returns the estuary_materialization_dekaf_singlestore resource.
var NewSingleStore func() resource.Resource = newResource[singleStore]("materialization_dekaf_singlestore")

// NewStarTree returns the estuary_materialization_dekaf_startree resource.
var NewStarTree func() resource.Resource = newResource[starTree]("materialization_dekaf_startree")

// NewImplyPolaris returns the estuary_materialization_dekaf_imply_polaris resource.
var NewImplyPolaris func() resource.Resource = newResource[implyPolaris]("materialization_dekaf_imply_polaris")

// NewBytewax returns the estuary_materialization_dekaf_bytewax resource.
var NewBytewax func() resource.Resource = newResource[bytewax]("materialization_dekaf_bytewax")
