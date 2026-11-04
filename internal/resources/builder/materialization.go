package builder

import (
	"context"
	"fmt"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder/translator"
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
	return newResource[materializationDocument[C, R], MaterializationSpec[C, R]](connector, materializationType[C, R]())
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

// movedTableNote documents that changing a binding's schema or table moves it, see warnMovedTables.
const movedTableNote string = " Changing it on an existing binding moves it to a new table, which the connector backfills " +
	"from the start of the collection, leaving the old table in place."

// materializationType returns the materialization catalog type of config C and binding resource R, whose x-schema-name
// field, if any, plans like the dashboard names it, defaulting to the C field tagged x-default-namespace.
func materializationType[C, R any]() catalogType {
	var schemaName string = translator.AttributeName(taggedKey[R](catalog.SchemaNameField))
	var tableName string = translator.AttributeName(taggedKey[R](catalog.CollectionNameField))
	var namespaceName string = translator.AttributeName(taggedKey[C](catalog.DefaultNamespaceField))
	var err error

	if schemaName != "" && namespaceName == "" {
		err = fmt.Errorf("the binding resource has an x-schema-name field, %s, but the config has no x-default-namespace field", schemaName)
	}

	return catalogType{
		name:            "materialization",
		err:             err,
		added:           materializationAddedBindings,
		publish:         publishMaterialization,
		deltaUpdatesKey: taggedKey[R](catalog.DeltaUpdatesField),
		adjustAttributes: func(attributes map[string]schema.Attribute) {
			adjustMaterializationAttributes(attributes, schemaName, tableName)
		},
		planBindings: func(plan tftypes.Value, config tftypes.Value, state tftypes.Value, readLive liveReader) (tftypes.Value, error) {
			planned, err := planBindingFields(plan, config, state, readLive)

			if err != nil || schemaName == "" {
				return planned, err
			}

			return planBindingSchemas(schemaName, planned, config, state, readLive)
		},
		movedTables: func(plan tftypes.Value, readLive liveReader) []string {
			if tableName == "" {
				return nil
			}

			return movedTables(schemaName, tableName, namespaceName, plan, readLive)
		},
		omittedDefault: materializationOmittedDefault,
	}
}

// adjustMaterializationAttributes makes binding fields and the resource schemaName attribute computed, as plans fill
// in the omitted ones, and documents that changing the schemaName or tableName attributes moves the binding.
func adjustMaterializationAttributes(attributes map[string]schema.Attribute, schemaName string, tableName string) {
	bindings, isList := attributes[bindingsAttribute].(schema.ListNestedAttribute)

	if !isList {
		return
	}

	if fields, isObject := bindings.NestedObject.Attributes[bindingFieldsAttribute].(schema.SingleNestedAttribute); isObject {
		fields.Computed = true
		fields.Description += " Omitted, it keeps the live selection, and new bindings get Estuary's default recommended = \"unlimited\"."

		bindings.NestedObject.Attributes[bindingFieldsAttribute] = fields
	}

	if resource, isObject := bindings.NestedObject.Attributes[bindingResourceAttribute].(schema.SingleNestedAttribute); isObject {
		if schemaAttribute, isString := resource.Attributes[schemaName].(schema.StringAttribute); isString {
			schemaAttribute.Computed = true
			schemaAttribute.Description += " Omitted, it keeps the live one, and a new binding gets the one target_naming " +
				"names, as in the dashboard, or the endpoint default without target_naming." + movedTableNote

			resource.Attributes[schemaName] = schemaAttribute
		}

		if tableAttribute, isString := resource.Attributes[tableName].(schema.StringAttribute); isString {
			tableAttribute.Description += movedTableNote

			resource.Attributes[tableName] = tableAttribute
		}

		bindings.NestedObject.Attributes[bindingResourceAttribute] = resource
	}

	attributes[bindingsAttribute] = bindings
}

// omittedAttribute is a binding attribute plans fill in when the configuration omits it.
type omittedAttribute struct {
	index    func(path *tftypes.AttributePath) (int, bool)                                           // binding index of a path to it
	value    func(binding tftypes.Value, attributeType tftypes.Type) tftypes.Value                   // its value in a binding
	fallback func(collection string, isRead bool, attributeType tftypes.Type) (tftypes.Value, error) // its value when neither state nor live holds it
}

// planOmitted plans the attribute of each binding configured without it as the one in state, else in the live spec for
// bindings Terraform takes over, else as its fallback, told whether the live spec was read.
func planOmitted(attribute omittedAttribute, plan tftypes.Value, config tftypes.Value, state tftypes.Value, readLive liveReader) (tftypes.Value, error) {
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

	var readLiveBindings func() (map[string]tftypes.Value, bool) = liveBindingsReader(readLive)

	return tftypes.Transform(plan, func(attributePath *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		index, isAttribute := attribute.index(attributePath)

		if !isAttribute || index >= len(configured) || index >= len(planned) || !attribute.value(configured[index], current.Type()).IsNull() {
			return current, nil
		}

		var collection string = bindingCollectionValue(planned[index])

		if kept, isStored := stored[collection]; isStored {
			return attribute.value(kept, current.Type()), nil
		}

		liveBindings, isRead := readLiveBindings()

		if kept, isLive := liveBindings[collection]; isLive && collection != "" {
			return attribute.value(kept, current.Type()), nil
		}

		return attribute.fallback(collection, isRead, current.Type())
	})
}

// planBindingFields plans omitted binding fields, Estuary's default recommended = "unlimited" for new bindings. Bindings
// of an existing entry whose live spec can't be read plan none, so an unread live selection is never replaced.
func planBindingFields(plan tftypes.Value, config tftypes.Value, state tftypes.Value, readLive liveReader) (tftypes.Value, error) {
	return planOmitted(omittedAttribute{
		index: func(path *tftypes.AttributePath) (int, bool) {
			return bindingAttributeIndex(path, bindingFieldsAttribute)
		},
		value: func(binding tftypes.Value, attributeType tftypes.Type) tftypes.Value {
			return blockAttribute(binding, bindingFieldsAttribute, attributeType)
		},
		fallback: func(_ string, isRead bool, attributeType tftypes.Type) (tftypes.Value, error) {
			if exists(state) && !isRead {
				return tftypes.NewValue(attributeType, nil), nil
			}

			return defaultBindingFields(attributeType)
		},
	}, plan, config, state, readLive)
}

// planBindingSchemas plans omitted resource schemaName attributes as the dashboard names a new binding from
// target_naming. They stay unknown while the collection or target_naming is, or while the live spec of an existing entry
// can't be read, so an unread live schema is never replaced.
func planBindingSchemas(schemaName string, plan tftypes.Value, config tftypes.Value, state tftypes.Value, readLive liveReader) (tftypes.Value, error) {
	naming, isKnown := targetNaming(plan)

	return planOmitted(omittedAttribute{
		index: func(path *tftypes.AttributePath) (int, bool) {
			return bindingResourceAttributeIndex(path, schemaName)
		},
		value: func(binding tftypes.Value, _ tftypes.Type) tftypes.Value {
			return resourceString(binding, schemaName)
		},
		fallback: func(collection string, isRead bool, _ tftypes.Type) (tftypes.Value, error) {
			if collection == "" || !isKnown || exists(state) && !isRead {
				return tftypes.NewValue(tftypes.String, tftypes.UnknownValue), nil
			}

			if schema := naming.SchemaName(collection); schema != "" {
				return tftypes.NewValue(tftypes.String, schema), nil
			}

			return tftypes.NewValue(tftypes.String, nil), nil
		},
	}, plan, config, state, readLive)
}

// movedTables lists the planned bindings whose table differs from their live one, as "collection: old -> new" lines.
// Names differing only in case are the same table.
func movedTables(schemaName string, tableName string, namespaceName string, plan tftypes.Value, readLive liveReader) []string {
	var moved []string

	planned, _, err := bindings(plan)

	if err != nil {
		return nil
	}

	live, found := readLive()

	if !found {
		return nil
	}

	liveBindings, err := indexedBindings(live)

	if err != nil {
		return nil
	}

	for _, binding := range planned {
		var collection string = bindingCollectionValue(binding)

		liveBinding, isLive := liveBindings[collection]

		if collection == "" || !isLive {
			continue
		}

		previous, isRead := bindingTable(schemaName, tableName, namespaceName, live, liveBinding)
		current, isKnown := bindingTable(schemaName, tableName, namespaceName, plan, binding)

		if isRead && isKnown && !strings.EqualFold(previous, current) {
			moved = append(moved, collection+": "+previous+" -> "+current)
		}
	}

	return moved
}

// bindingTable returns the schema-qualified table of a binding of entry, reporting false while unknown. A binding without
// schema uses the endpoint default, the config namespaceName attribute.
func bindingTable(schemaName string, tableName string, namespaceName string, entry tftypes.Value, binding tftypes.Value) (string, bool) {
	var schema tftypes.Value = tftypes.NewValue(tftypes.String, nil)

	table, isKnown := known[string](resourceString(binding, tableName))

	if !isKnown {
		return "", false
	}

	if schemaName != "" {
		schema = resourceString(binding, schemaName)
	}

	if schemaName != "" && schema.IsKnown() && schema.IsNull() {
		if endpointSchema, err := attributeValue(entry, configPath.WithAttributeName(namespaceName)); err == nil {
			schema = endpointSchema
		}
	}

	if !schema.IsKnown() {
		return "", false
	}

	if name, isNamed := known[string](schema); isNamed && name != "" {
		return name + "." + table, true
	}

	return table, true
}

// targetNaming returns the target_naming of value, reporting false while it's unknown.
func targetNaming(value tftypes.Value) (materialization.TargetNaming, bool) {
	naming, err := attributeValue(value, targetNamingPath)

	if err != nil || !naming.IsFullyKnown() {
		return materialization.TargetNaming{}, false
	}

	strategy, _ := known[string](blockAttribute(naming, "strategy", tftypes.String))
	schema, _ := known[string](blockAttribute(naming, "schema", tftypes.String))
	schemaTemplate, _ := known[string](blockAttribute(naming, "schema_template", tftypes.String))

	return materialization.TargetNaming{
		Strategy:       materialization.TargetNamingStrategy(strategy),
		Schema:         schema,
		SchemaTemplate: schemaTemplate,
	}, true
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

// NewMaterializationFromDocument returns the resource factory of a non-connector materialization, such as Dekaf. C and
// R are its config and resource, whose tags drive the materialization behaviors.
func NewMaterializationFromDocument[D Document[D, S], S catalog.MaterializationSpec, C, R any](connector Connector) func() resource.Resource {
	return newResource[D, S](connector, materializationType[C, R]())
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
