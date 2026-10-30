package builder

import (
	"context"
	"fmt"
	"sync"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// MaterializationSpec is the catalog spec of a connector materialization with config C and binding resource R.
type MaterializationSpec[C, R any] = materialization.Spec[endpoint.ConnectorSpec[C], R]

// materializationDocument is the Terraform shape of a connector materialization, plus a backfill block the catalog spec
// doesn't hold.
type materializationDocument[C, R any] struct {
	Name                        string `yaml:"name"`
	Image                       string `yaml:"image,omitempty"`
	materialization.SpecOptions `yaml:",inline"`
	Config                      C                           `yaml:"config" description:"Connection, authentication and default settings of the destination."`
	Bindings                    []MaterializationBinding[R] `yaml:"bindings" description:"Materialized collections, one per binding."`
	Backfill                    *MaterializationBackfill    `yaml:"backfill,omitempty" description:"Backfill control of the whole materialization."`
}

// MaterializationBinding is the Terraform shape of a materialization binding, with its source flattened beside the time
// filters. Exported for documents of non-connector endpoints.
type MaterializationBinding[R any] struct {
	Resource                   R                                          `yaml:"resource" description:"Destination resource the collection is materialized into, such as a table."`
	Source                     string                                     `yaml:"source" description:"Estuary collection the binding materializes. The provider tenant is not added to it. Changing it replaces the binding."`
	NotBefore                  string                                     `yaml:"notBefore,omitempty" description:"Materializes only the documents published at or after this RFC 3339 time. It only filters: changing it does not reprocess documents already read."`
	NotAfter                   string                                     `yaml:"notAfter,omitempty" description:"Materializes only the documents published before this RFC 3339 time. It only filters, like not_before."`
	Disable                    bool                                       `yaml:"disable,omitempty" description:"Stops materializing the collection while keeping the binding. Defaults to false."`
	Priority                   int                                        `yaml:"priority,omitempty" description:"Processing order among bindings: all ready documents of a higher-priority binding are processed before any of a lower one. Defaults to 0 and can be negative."`
	Fields                     *materialization.BindingFields             `yaml:"fields,omitempty" description:"Collection fields materialized as columns."`
	OnIncompatibleSchemaChange materialization.OnIncompatibleSchemaChange `yaml:"onIncompatibleSchemaChange,omitempty" enum:"backfill,disableBinding,disableTask,abort" description:"Overrides the on_incompatible_schema_change of the materialization for this binding: backfill, disableBinding, disableTask or abort."`
	Backfill                   *MaterializationBackfill                   `yaml:"backfill,omitempty" description:"Backfill control of the binding."`
}

func publishMaterialization(ctx context.Context, backend catalog.Backend, name string, document map[string]any, _ publication) error {
	return backend.PublishMaterialization(ctx, name, catalog.RawMaterialization(document))
}

// NewMaterialization returns the resource factory of a connector materialization.
func NewMaterialization[C, R any](connector Connector) func() resource.Resource {
	return NewMaterializationFromDocument[materializationDocument[C, R]](connector)
}

// materializationAddedBindings describes the bindings a source capture adds, which only get the source and target
// naming settings.
var materializationAddedBindings addedBindings = addedBindings{
	setting:   path.Root("source").AtName("capture"),
	generator: path.Root("source").AtName("capture"),
	summary:   "Source capture adds unmanaged bindings",
	origin:    "collection of the capture",
	growth:    "the capture adds new collections",
	limits: "Added bindings get only source.delta_updates, source.fields_recommended and target_naming. To give one per-binding " +
		"settings, such as fields.require, priority or on_incompatible_schema_change, declare it in bindings.",
	readding: "Estuary may add them back as long as the capture has their collections",
	stopping: "materializing",
}

// fieldsRecommendedPath is the path of source.fields_recommended.
var fieldsRecommendedPath *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName("source").WithAttributeName("fields_recommended")

// materializationOmittedDefault reports an unlimited source.fields_recommended, Estuary's default, which live specs omit.
func materializationOmittedDefault(path *tftypes.AttributePath, value tftypes.Value) bool {
	depth, isKnown := known[string](value)

	return isKnown && path.Equal(fieldsRecommendedPath) && materialization.FieldDepth(depth) == materialization.FieldDepthUnlimited
}

// materializationType is the materialization catalog type.
var materializationType catalogType = catalogType{
	name:             "materialization",
	added:            materializationAddedBindings,
	publish:          publishMaterialization,
	adjustAttributes: adjustMaterializationAttributes,
	planBindings:     planBindingFields,
	omittedDefault:   materializationOmittedDefault,
}

// adjustMaterializationAttributes makes binding fields computed, as plans fill in the omitted ones.
func adjustMaterializationAttributes(attributes map[string]schema.Attribute) {
	bindings, isList := attributes[bindingsAttribute].(schema.ListNestedAttribute)

	if !isList {
		return
	}

	if fields, isObject := bindings.NestedObject.Attributes[bindingFieldsAttribute].(schema.SingleNestedAttribute); isObject {
		fields.Computed = true
		fields.Description += " Omitted, it keeps the live selection, and new bindings get Estuary's default recommended = \"unlimited\"."

		bindings.NestedObject.Attributes[bindingFieldsAttribute] = fields
	}

	attributes[bindingsAttribute] = bindings
}

// planBindingFields plans the fields of each binding configured without them as the live ones in state, else in the live
// spec for bindings Terraform takes over, else Estuary's default recommended = "unlimited" for new bindings. Bindings of
// an existing entry whose live spec can't be read plan none, so an unread live selection is never replaced.
func planBindingFields(plan tftypes.Value, config tftypes.Value, state tftypes.Value, readLive liveReader) (tftypes.Value, error) {
	configured, _, err := bindings(config)

	if err != nil {
		return plan, err
	}

	planned, _, err := bindings(plan)

	if err != nil {
		return plan, err
	}

	stored, err := indexedBindings(state)

	if err != nil {
		return plan, err
	}

	var readLiveBindings func() (map[string]tftypes.Value, bool) = sync.OnceValues(func() (map[string]tftypes.Value, bool) {
		live, found := readLive()

		if !found {
			return nil, false
		}

		indexed, err := indexedBindings(live)

		return indexed, err == nil
	})

	return tftypes.Transform(plan, func(attributePath *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		index, isFields := bindingAttributeIndex(attributePath, bindingFieldsAttribute)

		if !isFields || index >= len(configured) || index >= len(planned) ||
			!blockAttribute(configured[index], bindingFieldsAttribute, current.Type()).IsNull() {
			return current, nil
		}

		var collection string = bindingCollectionValue(planned[index])

		if kept, isStored := stored[collection]; isStored {
			return blockAttribute(kept, bindingFieldsAttribute, current.Type()), nil
		}

		liveBindings, isRead := readLiveBindings()

		if kept, isLive := liveBindings[collection]; isLive && collection != "" {
			return blockAttribute(kept, bindingFieldsAttribute, current.Type()), nil
		}

		if exists(state) && !isRead {
			return tftypes.NewValue(current.Type(), nil), nil
		}

		return defaultBindingFields(current.Type())
	})
}

// defaultBindingFields returns fields selecting recommended = "unlimited", which specs write as Estuary's default true.
func defaultBindingFields(fieldsType tftypes.Type) (tftypes.Value, error) {
	var attributes map[string]tftypes.Value = map[string]tftypes.Value{}

	objectType, isObject := fieldsType.(tftypes.Object)

	if !isObject {
		return tftypes.Value{}, fmt.Errorf("binding fields are a %s, not an object", fieldsType)
	}

	recommendedType, found := objectType.AttributeTypes[bindingFieldsRecommendedAttribute]

	if !found {
		return tftypes.Value{}, fmt.Errorf("binding fields have no %s attribute", bindingFieldsRecommendedAttribute)
	}

	for attribute, attributeType := range objectType.AttributeTypes {
		attributes[attribute] = tftypes.NewValue(attributeType, nil)
	}

	attributes[bindingFieldsRecommendedAttribute] = tftypes.NewValue(recommendedType, string(materialization.FieldDepthUnlimited))

	return tftypes.NewValue(objectType, attributes), nil
}

// NewMaterializationFromDocument returns the resource factory of a non-connector materialization, such as Dekaf.
func NewMaterializationFromDocument[D Document[D, S], S catalog.MaterializationSpec](connector Connector) func() resource.Resource {
	return newResource[D, S](connector, materializationType)
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
