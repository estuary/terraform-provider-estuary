package builder

import (
	"context"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder/translator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// CaptureSpec is the catalog spec of a connector capture with config C and binding resource R.
type CaptureSpec[C, R any] = capture.Spec[endpoint.ConnectorSpec[C], R]

// captureDocument is the Terraform shape of a capture, plus a backfill block the catalog spec doesn't hold.
type captureDocument[C, R any] struct {
	Name         string                `yaml:"name"`
	Image        string                `yaml:"image,omitempty"`
	AutoDiscover *capture.AutoDiscover `yaml:"autoDiscover,omitempty" description:"Periodically checks the source for new resources and schema changes."`
	Config       C                     `yaml:"config" description:"Connection, authentication and capture settings of the source."`
	Bindings     []captureBinding[R]   `yaml:"bindings" description:"Captured resources, one per binding."`
	Shards       catalog.Shards        `yaml:"shards,omitempty" description:"Shard settings of the capture, with Estuary keys such as logLevel and disable."`
	Backfill     *captureBackfill      `yaml:"backfill,omitempty" description:"Backfill control of the whole capture."`
}

// captureBinding is the Terraform shape of a capture binding, with its backfill settings in a backfill block.
type captureBinding[R any] struct {
	Resource R                       `yaml:"resource" description:"Source resource the binding captures, such as a table or collection."`
	Target   string                  `yaml:"target"`
	Disable  bool                    `yaml:"disable,omitempty" description:"Stops capturing the resource while keeping the binding in the capture."`
	Backfill *captureBindingBackfill `yaml:"backfill,omitempty" description:"Backfill control of the binding."`
}

// captureAddedBindings describes the bindings auto discovery adds.
var captureAddedBindings addedBindings = addedBindings{
	setting:   path.Root("auto_discover").AtName("add_new_bindings"),
	generator: path.Root("auto_discover"),
	summary:   "Auto discovery adds unmanaged bindings",
	origin:    "discovered resource",
	growth:    "later discoveries find new ones",
	readding:  "the next discovery may add them back as long as their source resources exist",
	stopping:  "capturing",
}

// captureType returns the capture catalog type of binding resource R: it lists every live binding, and holds each
// binding backfill mode in its backfill block rather than in its resource.
func captureType[R any]() catalogType {
	var key string = taggedKey[R](catalog.BackfillModeField)

	return catalogType{
		name:             "capture",
		added:            captureAddedBindings,
		publish:          publishCapture,
		backfillModeKey:  key,
		adjustAttributes: adjustCaptureAttributes[R],
		toSpecShape: func(document map[string]any) {
			if key != "" {
				moveField(document[bindingsAttribute], backfillAttribute, backfillModeAttribute, bindingResourceAttribute, key)
			}
		},
		toTerraformShape: func(document map[string]any) {
			if key != "" {
				moveField(document[bindingsAttribute], bindingResourceAttribute, key, backfillAttribute, backfillModeAttribute)
			}
		},
	}
}

// NewCapture returns the resource factory of a connector capture.
func NewCapture[C, R any](connector Connector) func() resource.Resource {
	return newResource[captureDocument[C, R], CaptureSpec[C, R]](connector, captureType[R]())
}

// NewCaptureDataSource returns the data source factory reading the live bindings of a connector capture by name.
func NewCaptureDataSource[C, R any](connector Connector) func() datasource.DataSource {
	return newDataSource[captureDocument[C, R], CaptureSpec[C, R]](connector, captureType[R]())
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

// adjustCaptureAttributes moves the resource backfill mode into the binding backfill block, and makes the target
// optional when resources have a resource path.
func adjustCaptureAttributes[R any](attributes map[string]schema.Attribute) {
	var resource R

	bindings, isList := attributes[bindingsAttribute].(schema.ListNestedAttribute)

	if !isList {
		return
	}

	resourceAttribute, hasResource := bindings.NestedObject.Attributes[bindingResourceAttribute].(schema.SingleNestedAttribute)
	block, hasBlock := bindings.NestedObject.Attributes[backfillAttribute].(schema.SingleNestedAttribute)

	if hasResource && hasBlock {
		var key string = translator.AttributeName(taggedKey[R](catalog.BackfillModeField))

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

func publishCapture(ctx context.Context, backend catalog.Backend, name string, document map[string]any, publication publication) error {
	if publication.discover {
		return backend.PublishDiscoveredCapture(ctx, name, catalog.RawCapture(document), publication.identifier)
	}

	return backend.PublishCapture(ctx, name, catalog.RawCapture(document), publication.resets)
}
