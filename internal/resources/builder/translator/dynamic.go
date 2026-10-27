package translator

import (
	"fmt"
	"math/big"
	"reflect"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// dynamicNode handles a free-form spec map, such as shard settings, passing it through as written.
type dynamicNode struct{}

func isDynamic(mapType reflect.Type) bool {
	return mapType.Key().Kind() == reflect.String && mapType.Elem().Kind() == reflect.Interface
}

func (dynamicNode) attribute(field field) schema.Attribute {
	return schema.DynamicAttribute{
		Description: field.description,
		Required:    !field.optional,
		Optional:    field.optional,
	}
}

func (node dynamicNode) specValue(value tftypes.Value) (any, error) {
	switch {
	case value.IsNull():
		return nil, nil
	case value.Type().Is(tftypes.String):
		var text string

		err := value.As(&text)

		return text, err
	case value.Type().Is(tftypes.Bool):
		var flag bool

		err := value.As(&flag)

		return flag, err
	case value.Type().Is(tftypes.Number):
		var number big.Float

		if err := value.As(&number); err != nil {
			return nil, err
		}

		if integer, accuracy := number.Int64(); accuracy == big.Exact {
			return integer, nil
		}

		floating, _ := number.Float64()

		return floating, nil
	case value.Type().Is(tftypes.Object{}) || value.Type().Is(tftypes.Map{}):
		var entries map[string]tftypes.Value
		var object map[string]any = map[string]any{}

		if err := value.As(&entries); err != nil {
			return nil, err
		}

		for key, entry := range entries {
			specValue, err := node.specValue(entry)

			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}

			if specValue != nil {
				object[key] = specValue
			}
		}

		return object, nil
	default:
		var items []tftypes.Value
		var list []any = []any{}

		if err := value.As(&items); err != nil {
			return nil, fmt.Errorf("unsupported value type %s", value.Type())
		}

		for _, item := range items {
			specValue, err := node.specValue(item)

			if err != nil {
				return nil, err
			}

			list = append(list, specValue)
		}

		return list, nil
	}
}

// terraformValue converts a YAML value into a Terraform value of inferred type: objects for maps, tuples for lists.
func (node dynamicNode) terraformValue(value any, _ tftypes.Type) (tftypes.Value, error) {
	switch typed := value.(type) {
	case nil:
		return tftypes.NewValue(tftypes.DynamicPseudoType, nil), nil
	case string:
		return tftypes.NewValue(tftypes.String, typed), nil
	case bool:
		return tftypes.NewValue(tftypes.Bool, typed), nil
	case int:
		return tftypes.NewValue(tftypes.Number, big.NewFloat(float64(typed))), nil
	case int64:
		return tftypes.NewValue(tftypes.Number, new(big.Float).SetInt64(typed)), nil
	case uint64:
		return tftypes.NewValue(tftypes.Number, new(big.Float).SetUint64(typed)), nil
	case float64:
		return tftypes.NewValue(tftypes.Number, big.NewFloat(typed)), nil
	case map[string]any:
		var keys []string
		var attributes map[string]tftypes.Value = make(map[string]tftypes.Value, len(typed))
		var attributeTypes map[string]tftypes.Type = make(map[string]tftypes.Type, len(typed))

		for key := range typed {
			keys = append(keys, key)
		}

		sort.Strings(keys)

		for _, key := range keys {
			converted, err := node.terraformValue(typed[key], nil)

			if err != nil {
				return tftypes.Value{}, fmt.Errorf("%s: %w", key, err)
			}

			attributes[key] = converted
			attributeTypes[key] = converted.Type()
		}

		return tftypes.NewValue(tftypes.Object{AttributeTypes: attributeTypes}, attributes), nil
	case []any:
		var items []tftypes.Value
		var itemTypes []tftypes.Type

		for _, item := range typed {
			converted, err := node.terraformValue(item, nil)

			if err != nil {
				return tftypes.Value{}, err
			}

			items = append(items, converted)
			itemTypes = append(itemTypes, converted.Type())
		}

		return tftypes.NewValue(tftypes.Tuple{ElementTypes: itemTypes}, items), nil
	default:
		return tftypes.Value{}, fmt.Errorf("unsupported value %T", value)
	}
}
