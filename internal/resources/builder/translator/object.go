package translator

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// HashSuffix names the computed attribute beside each write-only attribute, holding the salted hash of its secret.
const HashSuffix string = "_hash"

type child struct {
	field     field
	converter node
}

// Object handles a spec struct, mapping each YAML key to its attribute through the field node.
type Object struct {
	children []child
}

// NewObject builds the node tree of a spec struct from its YAML tags.
func NewObject(structType reflect.Type) (*Object, error) {
	var object *Object = &Object{}
	var attributes []string

	mapped, err := fields(structType)

	if err != nil {
		return nil, err
	}

	for _, field := range mapped {
		converter, err := newNode(field.kind)

		if err != nil {
			return nil, fmt.Errorf("%s: %w", field.key, err)
		}

		object.children = append(object.children, child{
			field:     field,
			converter: converter,
		})

		attributes = append(attributes, field.attribute)
	}

	for _, child := range object.children {
		if child.converter.attribute(child.field).IsWriteOnly() && slices.Contains(attributes, child.field.attribute+HashSuffix) {
			return nil, fmt.Errorf("%s: attribute %s is the hash of the write-only %s.", structType, child.field.attribute+HashSuffix, child.field.attribute)
		}
	}

	return object, nil
}

// SpecValue converts a Terraform value of the struct into its YAML value, null becoming nil and the attributes outside
// the struct, such as computed ones still unknown, being ignored.
func (object *Object) SpecValue(value tftypes.Value) (any, error) {
	if value.IsNull() {
		return nil, nil
	}

	if !value.IsKnown() {
		return nil, errors.New("value is not known yet")
	}

	return object.specValue(value)
}

// TerraformValue converts a YAML value of the struct into its Terraform value, nil and the attributes outside the struct becoming null.
func (object *Object) TerraformValue(value any, valueType tftypes.Type) (tftypes.Value, error) {
	return toTerraformValue(object, value, valueType)
}

// Attributes returns the Terraform attributes of the struct fields, each write-only one along with its computed hash,
// whose changes plans show without storing the secret.
func (object *Object) Attributes() map[string]schema.Attribute {
	var built map[string]schema.Attribute = make(map[string]schema.Attribute, len(object.children))

	for _, child := range object.children {
		var attribute schema.Attribute = child.converter.attribute(child.field)

		built[child.field.attribute] = attribute

		if attribute.IsWriteOnly() {
			built[child.field.attribute+HashSuffix] = schema.StringAttribute{
				Description: "Salted hash of " + child.field.attribute + ", detecting its changes without storing it.",
				Computed:    true,
				Sensitive:   true,
			}
		}
	}

	return built
}

func (object *Object) attribute(field field) schema.Attribute {
	return schema.SingleNestedAttribute{
		Description: field.description,
		Required:    !field.optional,
		Optional:    field.optional,
		Attributes:  object.Attributes(),
	}
}

func (object *Object) specValue(value tftypes.Value) (any, error) {
	var attributes map[string]tftypes.Value
	var converted map[string]any = map[string]any{}

	if err := value.As(&attributes); err != nil {
		return nil, err
	}

	for _, child := range object.children {
		specValue, err := toSpecValue(child.converter, attributes[child.field.attribute])

		if err != nil {
			return nil, fmt.Errorf("%s: %w", child.field.attribute, err)
		}

		if specValue != nil {
			converted[child.field.key] = specValue
		}
	}

	return converted, nil
}

func (object *Object) terraformValue(value any, valueType tftypes.Type) (tftypes.Value, error) {
	var attributes map[string]tftypes.Value = make(map[string]tftypes.Value, len(object.children))

	values, isObject := value.(map[string]any)
	objectType, isObjectType := valueType.(tftypes.Object)

	if !isObject || !isObjectType {
		return tftypes.Value{}, fmt.Errorf("expected an object, got %T", value)
	}

	for _, child := range object.children {
		converted, err := toTerraformValue(child.converter, values[child.field.key], objectType.AttributeTypes[child.field.attribute])

		if err != nil {
			return tftypes.Value{}, fmt.Errorf("%s: %w", child.field.key, err)
		}

		attributes[child.field.attribute] = converted
	}

	for attribute, attributeType := range objectType.AttributeTypes {
		if _, found := attributes[attribute]; !found {
			attributes[attribute] = tftypes.NewValue(attributeType, nil)
		}
	}

	return tftypes.NewValue(valueType, attributes), nil
}
