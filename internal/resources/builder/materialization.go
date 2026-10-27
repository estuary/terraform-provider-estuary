package builder

import (
	"context"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// MaterializationSpec is the catalog spec of a connector materialization with config C and binding resource R.
type MaterializationSpec[C, R any] = materialization.Spec[endpoint.ConnectorSpec[C], R]

type materializationDocument[C, R any] struct {
	Name                        string `yaml:"name"`
	Image                       string `yaml:"image,omitempty"`
	materialization.SpecOptions `yaml:",inline"`
	Config                      C                           `yaml:"config"`
	Bindings                    []MaterializationBinding[R] `yaml:"bindings"`
}

// MaterializationBinding is the Terraform shape of a materialization binding, with its source flattened beside the time
// filters. Exported for documents of non-connector endpoints.
type MaterializationBinding[R any] struct {
	Resource                   R                                          `yaml:"resource"`
	Source                     string                                     `yaml:"source"`
	NotBefore                  string                                     `yaml:"notBefore,omitempty"`
	NotAfter                   string                                     `yaml:"notAfter,omitempty"`
	Disable                    bool                                       `yaml:"disable,omitempty"`
	Priority                   int                                        `yaml:"priority,omitempty"`
	Fields                     *materialization.BindingFields             `yaml:"fields,omitempty"`
	OnIncompatibleSchemaChange materialization.OnIncompatibleSchemaChange `yaml:"onIncompatibleSchemaChange,omitempty" enum:"backfill,disableBinding,disableTask,abort"`
	Backfill                   *MaterializationBackfill                   `yaml:"backfill,omitempty"`
}

func publishMaterialization(ctx context.Context, backend catalog.Backend, name string, document map[string]any, _ publication) error {
	return backend.PublishMaterialization(ctx, name, catalog.RawMaterialization(document))
}

// NewMaterialization returns the resource factory of a connector materialization.
func NewMaterialization[C, R any](connector Connector) func() resource.Resource {
	return NewMaterializationFromDocument[materializationDocument[C, R]](connector)
}

// NewMaterializationFromDocument returns the resource factory of a non-connector materialization, such as Dekaf.
func NewMaterializationFromDocument[D Document[D, S], S catalog.MaterializationSpec](connector Connector) func() resource.Resource {
	return newResource[D, S](connector, "materialization", publishMaterialization)
}

// Spec returns the materialization catalog name and spec, nesting image and config under endpoint.connector.
func (document materializationDocument[C, R]) Spec() (string, MaterializationSpec[C, R]) {
	return document.Name, MaterializationSpec[C, R]{
		Endpoint: endpoint.ConnectorSpec[C]{
			Connector: endpoint.Connector[C]{
				Image:  document.Image,
				Config: document.Config,
			},
		},
		SpecOptions: document.SpecOptions,
		Bindings:    MaterializationBindingsSpec(document.Bindings),
	}
}

// FromSpec returns the document of a materialization catalog name and spec.
func (materializationDocument[C, R]) FromSpec(name string, spec MaterializationSpec[C, R]) materializationDocument[C, R] {
	return materializationDocument[C, R]{
		Name:        name,
		Image:       spec.Endpoint.Connector.Image,
		SpecOptions: spec.SpecOptions,
		Config:      spec.Endpoint.Connector.Config,
		Bindings:    NewMaterializationBindings(spec.Bindings),
	}
}

// NewMaterializationBindings returns the Terraform shape of materialization bindings.
func NewMaterializationBindings[R any](bindings []materialization.Binding[R]) []MaterializationBinding[R] {
	var shaped []MaterializationBinding[R] = make([]MaterializationBinding[R], len(bindings))

	for index, binding := range bindings {
		shaped[index] = MaterializationBinding[R]{
			Resource:                   binding.Resource,
			Source:                     binding.Source.Name,
			NotBefore:                  binding.Source.NotBefore,
			NotAfter:                   binding.Source.NotAfter,
			Disable:                    binding.Disable,
			Priority:                   binding.Priority,
			Fields:                     binding.Fields,
			OnIncompatibleSchemaChange: binding.OnIncompatibleSchemaChange,
		}
	}

	return shaped
}

// MaterializationBindingsSpec converts materialization bindings from their Terraform shape to their catalog spec.
func MaterializationBindingsSpec[R any](bindings []MaterializationBinding[R]) []materialization.Binding[R] {
	var specs []materialization.Binding[R] = make([]materialization.Binding[R], len(bindings))

	for index, binding := range bindings {
		specs[index] = materialization.Binding[R]{
			Resource: binding.Resource,
			Source: materialization.BindingSource{
				Name:      binding.Source,
				NotBefore: binding.NotBefore,
				NotAfter:  binding.NotAfter,
			},
			BindingOptions: materialization.BindingOptions{
				Disable:                    binding.Disable,
				Priority:                   binding.Priority,
				Fields:                     binding.Fields,
				OnIncompatibleSchemaChange: binding.OnIncompatibleSchemaChange,
			},
		}
	}

	return specs
}
