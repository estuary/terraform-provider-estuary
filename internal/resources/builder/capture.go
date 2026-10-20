package builder

import (
	"context"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// CaptureSpec is the catalog spec of a connector capture, C being its config and R its binding resource.
type CaptureSpec[C, R any] = capture.Spec[endpoint.ConnectorSpec[C], R]

// CaptureCodec converts a connector capture between its Terraform values and its catalog spec.
type CaptureCodec[C, R any] = Codec[captureDocument[C, R], CaptureSpec[C, R]]

type captureDocument[C, R any] struct {
	Name         string                `yaml:"name"`
	Image        string                `yaml:"image,omitempty"`
	AutoDiscover *capture.AutoDiscover `yaml:"autoDiscover,omitempty"`
	Config       C                     `yaml:"config"`
	Bindings     []capture.Binding[R]  `yaml:"bindings"`
}

// NewCapture returns the resource factory of a connector capture, registered in the provider resources.
func NewCapture[C, R any](connector Connector) func() resource.Resource {
	return newResource[captureDocument[C, R], CaptureSpec[C, R]](connector, "capture", publishCapture, true)
}

// NewCaptureCodec builds the codec of a connector capture.
func NewCaptureCodec[C, R any](connector Connector) (*CaptureCodec[C, R], error) {
	return NewCodec[captureDocument[C, R]](connector)
}

// Spec returns the capture catalog name and spec, nesting image and config under endpoint.connector.
func (document captureDocument[C, R]) Spec() (string, CaptureSpec[C, R]) {
	return document.Name, CaptureSpec[C, R]{
		AutoDiscover: document.AutoDiscover,
		Endpoint: endpoint.ConnectorSpec[C]{
			Connector: endpoint.Connector[C]{
				Image:  document.Image,
				Config: document.Config,
			},
		},
		Bindings: document.Bindings,
	}
}

// FromSpec returns the document of a capture catalog name and spec.
func (captureDocument[C, R]) FromSpec(name string, spec CaptureSpec[C, R]) captureDocument[C, R] {
	return captureDocument[C, R]{
		Name:         name,
		Image:        spec.Endpoint.Connector.Image,
		AutoDiscover: spec.AutoDiscover,
		Config:       spec.Endpoint.Connector.Config,
		Bindings:     spec.Bindings,
	}
}

// adjustAttributes makes the binding target optional when the resources recommend their collection name, see planTargets.
func (captureDocument[C, R]) adjustAttributes(attributes map[string]schema.Attribute) {
	var resource R

	bindings, isList := attributes[bindingsAttribute].(schema.ListNestedAttribute)

	if _, isRecommender := any(resource).(capture.NameRecommender); !isRecommender || !isList {
		return
	}

	bindings.NestedObject.Attributes[bindingTargetAttribute] = schema.StringAttribute{
		Description: "Collection the binding captures into, defaulting to the one discovery names: " +
			"the capture name without its connector name, followed by the resource recommended name.",
		Optional: true,
		Computed: true,
	}

	attributes[bindingsAttribute] = bindings
}

func publishCapture(ctx context.Context, backend catalog.Backend, name string, document map[string]any) error {
	return backend.PublishCapture(ctx, name, catalog.RawCapture(document))
}
