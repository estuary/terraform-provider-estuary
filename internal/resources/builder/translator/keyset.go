package translator

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// keySetNode handles a spec map of string keys to empty objects, such as required fields, as a set of its keys.
type keySetNode struct{}

func isKeySet(mapType reflect.Type) bool {
	return mapType.Key().Kind() == reflect.String && mapType.Elem().Kind() == reflect.Struct && mapType.Elem().NumField() == 0
}

func (keySetNode) attribute(field field) schema.Attribute {
	return schema.SetAttribute{
		Description: field.description,
		Required:    !field.optional,
		Optional:    field.optional,
		ElementType: types.StringType,
	}
}

func (keySetNode) specValue(value tftypes.Value) (any, error) {
	var items []tftypes.Value
	var converted map[string]any = map[string]any{}

	if err := value.As(&items); err != nil {
		return nil, err
	}

	for _, item := range items {
		var key string

		if err := item.As(&key); err != nil {
			return nil, err
		}

		converted[key] = map[string]any{}
	}

	return converted, nil
}

func (keySetNode) terraformValue(value any, valueType tftypes.Type) (tftypes.Value, error) {
	var keys []string
	var items []tftypes.Value

	entries, isMap := value.(map[string]any)
	setType, isSetType := valueType.(tftypes.Set)

	if !isMap || !isSetType {
		return tftypes.Value{}, fmt.Errorf("expected a map, got %T", value)
	}

	for key := range entries {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	for _, key := range keys {
		items = append(items, tftypes.NewValue(setType.ElementType, key))
	}

	return tftypes.NewValue(valueType, items), nil
}
