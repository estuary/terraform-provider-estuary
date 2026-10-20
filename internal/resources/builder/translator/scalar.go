package translator

import (
	"fmt"
	"math"
	"math/big"

	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// scalarNode is a node of a single value, which can also be the item of a plain list.
type scalarNode interface {
	node
	elementType() attr.Type
}

// stringNode handles string fields, including named string types such as enums.
type stringNode struct{}

// boolNode handles bool fields.
type boolNode struct{}

// integerNode handles signed and unsigned int fields, as Terraform Int64.
type integerNode struct{}

// floatNode handles float fields, as Terraform Float64.
type floatNode struct{}

func (stringNode) attribute(field field) schema.Attribute {
	var secret bool = utils.IsSensitiveKey(field.key)

	return schema.StringAttribute{
		Description: field.description,
		Required:    !field.optional,
		Optional:    field.optional,
		Sensitive:   secret,
		WriteOnly:   secret,
	}
}

func (stringNode) elementType() attr.Type {
	return types.StringType
}

func (stringNode) specValue(value tftypes.Value) (any, error) {
	var text string

	err := value.As(&text)

	return text, err
}

func (stringNode) terraformValue(value any, valueType tftypes.Type) (tftypes.Value, error) {
	text, isString := value.(string)

	if !isString {
		return tftypes.Value{}, fmt.Errorf("expected a string, got %T", value)
	}

	return tftypes.NewValue(valueType, text), nil
}

func (boolNode) attribute(field field) schema.Attribute {
	return schema.BoolAttribute{
		Description: field.description,
		Required:    !field.optional,
		Optional:    field.optional,
	}
}

func (boolNode) elementType() attr.Type {
	return types.BoolType
}

func (boolNode) specValue(value tftypes.Value) (any, error) {
	var flag bool

	err := value.As(&flag)

	return flag, err
}

func (boolNode) terraformValue(value any, valueType tftypes.Type) (tftypes.Value, error) {
	flag, isBool := value.(bool)

	if !isBool {
		return tftypes.Value{}, fmt.Errorf("expected a bool, got %T", value)
	}

	return tftypes.NewValue(valueType, flag), nil
}

func (integerNode) attribute(field field) schema.Attribute {
	return schema.Int64Attribute{
		Description: field.description,
		Required:    !field.optional,
		Optional:    field.optional,
	}
}

func (integerNode) elementType() attr.Type {
	return types.Int64Type
}

func (integerNode) specValue(value tftypes.Value) (any, error) {
	var number big.Float

	if err := value.As(&number); err != nil {
		return nil, err
	}

	if !number.IsInt() {
		return nil, fmt.Errorf("expected a whole number, got %s", number.String())
	}

	integer, accuracy := number.Int64()

	if accuracy != big.Exact {
		return nil, fmt.Errorf("%s overflows a 64-bit integer", number.String())
	}

	return integer, nil
}

func (integerNode) terraformValue(value any, valueType tftypes.Type) (tftypes.Value, error) {
	number, err := bigFloat(value)

	if err != nil {
		return tftypes.Value{}, err
	}

	return tftypes.NewValue(valueType, number), nil
}

func (floatNode) attribute(field field) schema.Attribute {
	return schema.Float64Attribute{
		Description: field.description,
		Required:    !field.optional,
		Optional:    field.optional,
	}
}

func (floatNode) elementType() attr.Type {
	return types.Float64Type
}

func (floatNode) specValue(value tftypes.Value) (any, error) {
	var number big.Float

	if err := value.As(&number); err != nil {
		return nil, err
	}

	decimal, _ := number.Float64()

	if math.IsInf(decimal, 0) {
		return nil, fmt.Errorf("%s overflows a 64-bit float", number.String())
	}

	return decimal, nil
}

func (floatNode) terraformValue(value any, valueType tftypes.Type) (tftypes.Value, error) {
	number, err := bigFloat(value)

	if err != nil {
		return tftypes.Value{}, err
	}

	return tftypes.NewValue(valueType, number), nil
}

// bigFloat converts a YAML decoded number, which decodes whole floats as ints, into a Terraform number.
func bigFloat(value any) (*big.Float, error) {
	switch number := value.(type) {
	case int:
		return new(big.Float).SetInt64(int64(number)), nil
	case int64:
		return new(big.Float).SetInt64(number), nil
	case float64:
		return new(big.Float).SetFloat64(number), nil
	default:
		return nil, fmt.Errorf("expected a number, got %T", value)
	}
}
