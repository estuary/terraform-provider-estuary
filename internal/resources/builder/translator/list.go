package translator

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// listNode handles a spec slice, converting each item through the item node. Its attribute is a list, or a set when
// the schema makes it one, such as bindings, whose items keep their order.
type listNode struct {
	item node
}

func newListNode(kind reflect.Type) (*listNode, error) {
	item, err := newNode(kind)

	if err != nil {
		return nil, err
	}

	switch item.(type) {
	case *Object, scalarNode:
		return &listNode{item: item}, nil
	default:
		return nil, fmt.Errorf("unsupported list item type %s", kind)
	}
}

// attribute returns a nested list for struct items and a plain list for scalar items.
func (list *listNode) attribute(field field) schema.Attribute {
	if object, isObject := list.item.(*Object); isObject {
		return schema.ListNestedAttribute{
			Description: field.description,
			Required:    !field.optional,
			Optional:    field.optional,
			NestedObject: schema.NestedAttributeObject{
				Attributes: object.Attributes(),
			},
		}
	}

	return schema.ListAttribute{
		Description: field.description,
		Required:    !field.optional,
		Optional:    field.optional,
		ElementType: list.item.(scalarNode).elementType(),
	}
}

func (list *listNode) specValue(value tftypes.Value) (any, error) {
	var items []tftypes.Value
	var converted []any

	if err := value.As(&items); err != nil {
		return nil, err
	}

	for index, item := range items {
		specValue, err := toSpecValue(list.item, item)

		if err != nil {
			return nil, fmt.Errorf("item %d: %w", index, err)
		}

		converted = append(converted, specValue)
	}

	return converted, nil
}

func (list *listNode) terraformValue(value any, valueType tftypes.Type) (tftypes.Value, error) {
	var converted []tftypes.Value
	var elementType tftypes.Type

	items, isList := value.([]any)

	switch typed := valueType.(type) {
	case tftypes.List:
		elementType = typed.ElementType
	case tftypes.Set:
		elementType = typed.ElementType
	}

	if !isList || elementType == nil {
		return tftypes.Value{}, fmt.Errorf("expected a list, got %T", value)
	}

	for index, item := range items {
		terraformValue, err := toTerraformValue(list.item, item, elementType)

		if err != nil {
			return tftypes.Value{}, fmt.Errorf("item %d: %w", index, err)
		}

		converted = append(converted, terraformValue)
	}

	return tftypes.NewValue(valueType, converted), nil
}
