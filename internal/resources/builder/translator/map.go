package translator

import (
	"fmt"
	"reflect"

	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// mapNode handles a spec map of string keys to scalars, such as table properties, converting each value through the value node.
type mapNode struct {
	value scalarNode
}

func newMapNode(mapType reflect.Type) (*mapNode, error) {
	if mapType.Key().Kind() != reflect.String {
		return nil, fmt.Errorf("unsupported map key type %s", mapType.Key())
	}

	value, err := newNode(mapType.Elem())

	if err != nil {
		return nil, err
	}

	scalar, isScalar := value.(scalarNode)

	if !isScalar {
		return nil, fmt.Errorf("unsupported map value type %s", mapType.Elem())
	}

	return &mapNode{value: scalar}, nil
}

func (mapped *mapNode) attribute(field field) schema.Attribute {
	return schema.MapAttribute{
		Description: field.description,
		Required:    !field.optional,
		Optional:    field.optional,
		Sensitive:   utils.IsSensitiveKey(field.key),
		ElementType: mapped.value.elementType(),
	}
}

func (mapped *mapNode) specValue(value tftypes.Value) (any, error) {
	var entries map[string]tftypes.Value
	var converted map[string]any = map[string]any{}

	if err := value.As(&entries); err != nil {
		return nil, err
	}

	for key, entry := range entries {
		specValue, err := toSpecValue(mapped.value, entry)

		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}

		converted[key] = specValue
	}

	return converted, nil
}

func (mapped *mapNode) terraformValue(value any, valueType tftypes.Type) (tftypes.Value, error) {
	var converted map[string]tftypes.Value = map[string]tftypes.Value{}

	entries, isMap := value.(map[string]any)
	mapType, isMapType := valueType.(tftypes.Map)

	if !isMap || !isMapType {
		return tftypes.Value{}, fmt.Errorf("expected a map, got %T", value)
	}

	for key, entry := range entries {
		terraformValue, err := toTerraformValue(mapped.value, entry, mapType.ElementType)

		if err != nil {
			return tftypes.Value{}, fmt.Errorf("%s: %w", key, err)
		}

		converted[key] = terraformValue
	}

	return tftypes.NewValue(valueType, converted), nil
}
