package builder

import (
	"context"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder/translator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// CaptureSpec is the catalog spec of a connector capture with config C and binding resource R.
type CaptureSpec[C, R any] = capture.Spec[endpoint.ConnectorSpec[C], R]

// captureDocument is the Terraform shape of a capture, plus a backfill block the catalog spec doesn't hold.
type captureDocument[C, R any] struct {
	Name         string                `yaml:"name"`
	Image        string                `yaml:"image,omitempty"`
	AutoDiscover *capture.AutoDiscover `yaml:"autoDiscover,omitempty"`
	Config       C                     `yaml:"config"`
	Bindings     []captureBinding[R]   `yaml:"bindings"`
	Shards       catalog.Shards        `yaml:"shards,omitempty"`
	Backfill     *captureBackfill      `yaml:"backfill,omitempty"`
}

// captureBinding is the Terraform shape of a capture binding, with its backfill settings in a backfill block.
type captureBinding[R any] struct {
	Resource R                       `yaml:"resource"`
	Target   string                  `yaml:"target"`
	Disable  bool                    `yaml:"disable,omitempty"`
	Backfill *captureBindingBackfill `yaml:"backfill,omitempty"`
}

// NewCapture returns the resource factory of a connector capture.
func NewCapture[C, R any](connector Connector) func() resource.Resource {
	return newResource[captureDocument[C, R], CaptureSpec[C, R]](connector, "capture", publishCapture)
}

// Spec returns the capture catalog name and spec, nesting image and config under endpoint.connector.
func (document captureDocument[C, R]) Spec() (string, CaptureSpec[C, R]) {
	var bindings []capture.Binding[R] = make([]capture.Binding[R], len(document.Bindings))

	for index, binding := range document.Bindings {
		bindings[index] = capture.Binding[R]{
			Resource: binding.Resource,
			Target:   binding.Target,
			Disable:  binding.Disable,
		}
	}

	return document.Name, CaptureSpec[C, R]{
		AutoDiscover: document.AutoDiscover,
		Endpoint: endpoint.ConnectorSpec[C]{
			Connector: endpoint.Connector[C]{
				Image:  document.Image,
				Config: document.Config,
			},
		},
		Bindings: bindings,
		Shards:   document.Shards,
	}
}

// FromSpec returns the document of a capture catalog name and spec, without backfill blocks.
func (captureDocument[C, R]) FromSpec(name string, spec CaptureSpec[C, R]) captureDocument[C, R] {
	var bindings []captureBinding[R] = make([]captureBinding[R], len(spec.Bindings))

	for index, binding := range spec.Bindings {
		bindings[index] = captureBinding[R]{
			Resource: binding.Resource,
			Target:   binding.Target,
			Disable:  binding.Disable,
		}
	}

	return captureDocument[C, R]{
		Name:         name,
		Image:        spec.Endpoint.Connector.Image,
		AutoDiscover: spec.AutoDiscover,
		Config:       spec.Endpoint.Connector.Config,
		Bindings:     bindings,
		Shards:       spec.Shards,
	}
}

// adjustAttributes moves the resource backfill mode into the binding backfill block, and makes the target optional
// when resources have a resource path.
func (captureDocument[C, R]) adjustAttributes(attributes map[string]schema.Attribute) {
	var resource R

	bindings, isList := attributes[bindingsAttribute].(schema.ListNestedAttribute)

	if !isList {
		return
	}

	resourceAttribute, hasResource := bindings.NestedObject.Attributes["resource"].(schema.SingleNestedAttribute)
	block, hasBlock := bindings.NestedObject.Attributes[backfillAttribute].(schema.SingleNestedAttribute)

	if hasResource && hasBlock {
		var key string = translator.AttributeName(backfillModeKey[R]())

		delete(block.Attributes, backfillModeAttribute)

		if mode, found := resourceAttribute.Attributes[key]; found && key != "" {
			block.Attributes[backfillModeAttribute] = mode

			delete(resourceAttribute.Attributes, key)
		}
	}

	if _, isPather := any(resource).(capture.ResourcePather); isPather {
		bindings.NestedObject.Attributes[bindingTargetAttribute] = schema.StringAttribute{
			Description: "Collection the binding captures into, defaulting to its current one, else to the one " +
				"the connector discovery names at plan time.",
			Optional: true,
			Computed: true,
		}
	}

	attributes[bindingsAttribute] = bindings
}

// resourceBackfillMode returns the spec key of the resource backfill mode, empty without one.
func (captureDocument[C, R]) resourceBackfillMode() string {
	return backfillModeKey[R]()
}

// toSpecShape moves each binding backfill mode from its backfill block into its resource.
func (captureDocument[C, R]) toSpecShape(document map[string]any) {
	if key := backfillModeKey[R](); key != "" {
		moveField(document[bindingsAttribute], backfillAttribute, backfillModeAttribute, "resource", key)
	}
}

// toTerraformShape moves each binding backfill mode from its resource into its backfill block.
func (captureDocument[C, R]) toTerraformShape(document map[string]any) {
	if key := backfillModeKey[R](); key != "" {
		moveField(document[bindingsAttribute], "resource", key, backfillAttribute, backfillModeAttribute)
	}
}

func publishCapture(ctx context.Context, backend catalog.Backend, name string, document map[string]any, publication publication) error {
	if publication.discover {
		return backend.PublishDiscoveredCapture(ctx, name, catalog.RawCapture(document), publication.identifier)
	}

	return backend.PublishCapture(ctx, name, catalog.RawCapture(document), publication.resets)
}
