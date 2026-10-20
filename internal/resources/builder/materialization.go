package builder

import (
	"context"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// MaterializationSpec is the catalog spec of a connector materialization, C being its config and R its binding resource.
type MaterializationSpec[C, R any] = materialization.Spec[endpoint.ConnectorSpec[C], R]

// MaterializationCodec converts a connector materialization between its Terraform values and its catalog spec.
type MaterializationCodec[C, R any] = Codec[materializationDocument[C, R], MaterializationSpec[C, R]]

type materializationDocument[C, R any] struct {
	Name     string                       `yaml:"name"`
	Image    string                       `yaml:"image,omitempty"`
	Source   *materialization.Source      `yaml:"source,omitempty"`
	Config   C                            `yaml:"config"`
	Bindings []materialization.Binding[R] `yaml:"bindings"`
}

func publishMaterialization(ctx context.Context, backend catalog.Backend, name string, document map[string]any) error {
	return backend.PublishMaterialization(ctx, name, catalog.RawMaterialization(document))
}

// NewMaterialization returns the resource factory of a connector materialization, registered in the provider resources.
func NewMaterialization[C, R any](connector Connector) func() resource.Resource {
	return NewMaterializationFromDocument[materializationDocument[C, R]](connector)
}

// NewMaterializationFromDocument returns the resource factory of a materialization whose endpoint is not a connector
// image, such as Dekaf, D being the document mapping its Terraform shape to its spec S.
func NewMaterializationFromDocument[D Document[D, S], S catalog.MaterializationSpec](connector Connector) func() resource.Resource {
	return newResource[D, S](connector, "materialization", publishMaterialization, false)
}

// NewMaterializationCodec builds the codec of a connector materialization.
func NewMaterializationCodec[C, R any](connector Connector) (*MaterializationCodec[C, R], error) {
	return NewCodec[materializationDocument[C, R]](connector)
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
		Source:   document.Source,
		Bindings: document.Bindings,
	}
}

// FromSpec returns the document of a materialization catalog name and spec.
func (materializationDocument[C, R]) FromSpec(name string, spec MaterializationSpec[C, R]) materializationDocument[C, R] {
	return materializationDocument[C, R]{
		Name:     name,
		Image:    spec.Endpoint.Connector.Image,
		Source:   spec.Source,
		Config:   spec.Endpoint.Connector.Config,
		Bindings: spec.Bindings,
	}
}
