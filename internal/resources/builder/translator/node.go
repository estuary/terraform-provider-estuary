package translator

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type node interface {
	attribute(field field) schema.Attribute
	specValue(value tftypes.Value) (any, error)
	terraformValue(value any, valueType tftypes.Type) (tftypes.Value, error)
}

func newNode(kind reflect.Type) (node, error) {
	switch kind.Kind() {
	case reflect.Pointer:
		return newNode(kind.Elem())
	case reflect.Struct:
		return NewObject(kind)
	case reflect.Slice:
		return newListNode(kind.Elem())
	case reflect.Map:
		if isKeySet(kind) {
			return keySetNode{}, nil
		}

		return newMapNode(kind)
	case reflect.String:
		return stringNode{}, nil
	case reflect.Bool:
		return boolNode{}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return integerNode{}, nil
	case reflect.Float32, reflect.Float64:
		return floatNode{}, nil
	default:
		return nil, fmt.Errorf("unsupported field type %s", kind)
	}
}

func toSpecValue(converter node, value tftypes.Value) (any, error) {
	if value.IsNull() {
		return nil, nil
	}

	if !value.IsFullyKnown() {
		return nil, errors.New("value is not known yet")
	}

	return converter.specValue(value)
}

func toTerraformValue(converter node, value any, valueType tftypes.Type) (tftypes.Value, error) {
	if value == nil {
		return tftypes.NewValue(valueType, nil), nil
	}

	return converter.terraformValue(value, valueType)
}
