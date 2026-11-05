package translator

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"gopkg.in/yaml.v3"
)

// scalarNode is a node of a single value, also usable as a plain list item.
type scalarNode interface {
	node
	elementType() attr.Type
}

// stringNode handles string fields, including named string types such as enums.
type stringNode struct{}

// yamlStringNode handles named string types written as other YAML scalars, such as a depth written as a number.
type yamlStringNode struct {
	stringNode
	kind reflect.Type
}

// boolNode handles bool fields.
type boolNode struct{}

// integerNode handles signed and unsigned int fields, as Terraform Int64.
type integerNode struct{}

// floatNode handles float fields, as Terraform Float64.
type floatNode struct{}

func (stringNode) attribute(field field) schema.Attribute {
	var validators []validator.String

	if field.enum != nil {
		validators = append(validators, stringvalidator.OneOf(field.enum...))
	}

	if field.credentials != catalog.CredentialsAllowed {
		validators = append(validators, credentialsValidator{credentials: field.credentials})
	}

	// A declared secret must hold a value, so an empty variable never publishes an entry without its credential.
	// Leaving the attribute out stays valid, publishing the entry without it.
	if field.secret {
		validators = append(validators, stringvalidator.LengthAtLeast(1))
	}

	return schema.StringAttribute{
		Description: field.description,
		Required:    !field.optional,
		Optional:    field.optional,
		Sensitive:   field.secret,
		WriteOnly:   field.secret,
		Validators:  validators,
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

func (node yamlStringNode) attribute(field field) schema.Attribute {
	var attribute schema.StringAttribute = node.stringNode.attribute(field).(schema.StringAttribute)

	attribute.Validators = append(attribute.Validators, yamlStringValidator{kind: node.kind})

	return attribute
}

// yamlStringValidator rejects the values a named string type can't decode, such as a depth that isn't a number.
type yamlStringValidator struct {
	kind reflect.Type
}

func (yamlStringValidator) Description(context.Context) string {
	return "value must decode as the field type"
}

func (check yamlStringValidator) MarkdownDescription(ctx context.Context) string {
	return check.Description(ctx)
}

func (check yamlStringValidator) ValidateString(_ context.Context, validateReq validator.StringRequest, validateResp *validator.StringResponse) {
	if validateReq.ConfigValue.IsNull() || validateReq.ConfigValue.IsUnknown() {
		return
	}

	var node yaml.Node = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: validateReq.ConfigValue.ValueString()}

	if err := node.Decode(reflect.New(check.kind).Interface()); err != nil {
		validateResp.Diagnostics.AddAttributeError(validateReq.Path, "Invalid value", err.Error())
	}
}

// redacted replaces each credential RedactCredentials removes.
const redacted string = "***"

// credentialsValidator rejects connection URIs holding the credentials its field forbids, which state and logs would
// hold in plain text.
type credentialsValidator struct {
	credentials catalog.Credentials
}

func (check credentialsValidator) Description(context.Context) string {
	if check.credentials == catalog.CredentialsPassword {
		return "value must hold only the user, host and connection options, without password"
	}

	return "value must hold only the host and connection options, without credentials"
}

func (check credentialsValidator) MarkdownDescription(ctx context.Context) string {
	return check.Description(ctx)
}

func (check credentialsValidator) ValidateString(_ context.Context, validateReq validator.StringRequest, validateResp *validator.StringResponse) {
	if validateReq.ConfigValue.IsNull() || validateReq.ConfigValue.IsUnknown() {
		return
	}

	if _, holds := redactCredentials(validateReq.ConfigValue.ValueString(), check.credentials); !holds {
		return
	}

	if check.credentials == catalog.CredentialsPassword {
		validateResp.Diagnostics.AddAttributeError(validateReq.Path, "Password in connection string",
			"A password, such as user:password@ or a password option, would be stored in plain text in state and logs. "+
				"Keep only the user, as in user@hostname, and authenticate with the connector's write-only attributes instead, such as private_key.")

		return
	}

	validateResp.Diagnostics.AddAttributeError(validateReq.Path, "Credentials in connection string",
		"Credentials, such as user:password@ or a password or token option, would be stored in plain text in state and logs. "+
			"Set them in the connector's write-only attributes instead, such as password or a header value.")
}

// AttributeCredentials returns which credentials a schema attribute rejects in connection URIs, CredentialsAllowed when
// it rejects none.
func AttributeCredentials(attribute schema.Attribute) catalog.Credentials {
	text, isString := attribute.(schema.StringAttribute)

	if !isString {
		return catalog.CredentialsAllowed
	}

	for _, check := range text.Validators {
		if credentials, isCredentials := check.(credentialsValidator); isCredentials {
			return credentials.credentials
		}
	}

	return catalog.CredentialsAllowed
}

// uriScheme matches the scheme starting a URI, such as mongodb+srv://.
var uriScheme *regexp.Regexp = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// RedactCredentials returns a connection URI, with or without scheme, with the credentials its field forbids replaced
// by ***: the whole user info, or only its password with CredentialsPassword, and the values of credential options.
// Any @ counts as user info, as hosts and options never hold one.
func RedactCredentials(value string, credentials catalog.Credentials) string {
	safe, _ := redactCredentials(value, credentials)

	return safe
}

// HoldsCredentials reports whether a connection URI holds the credentials its field forbids, as the plan validator does.
func HoldsCredentials(value string, credentials catalog.Credentials) bool {
	_, holds := redactCredentials(value, credentials)

	return holds
}

// redactCredentials returns RedactCredentials of value, and whether value holds credentials, *** placeholders
// included, so a redacted value is never accepted as configuration.
func redactCredentials(value string, credentials catalog.Credentials) (string, bool) {
	var start int = len(uriScheme.FindString(value))
	var holds bool

	if at := strings.LastIndex(value, "@"); at >= 0 {
		start = min(start, at)

		if credentials == catalog.CredentialsPassword {
			if user, _, hasPassword := strings.Cut(value[start:at], ":"); hasPassword {
				holds = true

				value = value[:start] + user + ":" + redacted + value[at:]
			}
		} else {
			holds = true

			value = value[:start] + redacted + value[at:]
		}
	}

	base, options, hasOptions := strings.Cut(value, "?")

	if !hasOptions {
		return value, holds
	}

	options, holdsOptions := redactOptions(options)

	return base + "?" + options, holds || holdsOptions
}

// redactOptions replaces the values of credential options, split on & or ;, including credential properties of
// authMechanismProperties, such as AWS_SESSION_TOKEN. A value runs to the next separator, a # fragment included.
func redactOptions(options string) (string, bool) {
	var redactedOptions strings.Builder
	var start int
	var holds bool

	for index := 0; index <= len(options); index++ {
		if index < len(options) && options[index] != '&' && options[index] != ';' {
			continue
		}

		option, isCredential := redactOption(options[start:index])

		redactedOptions.WriteString(option)

		holds = holds || isCredential

		if index < len(options) {
			redactedOptions.WriteByte(options[index])
		}

		start = index + 1
	}

	return redactedOptions.String(), holds
}

// redactOption replaces the value of a key=value option naming a credential, reporting whether it names one.
func redactOption(option string) (string, bool) {
	key, value, hasValue := strings.Cut(option, "=")
	decodedKey, err := url.QueryUnescape(key)

	if err != nil {
		decodedKey = key
	}

	if !hasValue {
		return option, false
	}

	if catalog.IsSensitiveKey(decodedKey) {
		return key + "=" + redacted, true
	}

	if !strings.EqualFold(decodedKey, "authMechanismProperties") {
		return option, false
	}

	decodedValue, err := url.QueryUnescape(value)

	if err != nil {
		decodedValue = value
	}

	var properties []string = strings.Split(decodedValue, ",")
	var holds bool

	for index, property := range properties {
		if name, _, _ := strings.Cut(property, ":"); catalog.IsSensitiveKey(name) {
			properties[index] = name + ":" + redacted
			holds = true
		}
	}

	if !holds {
		return option, false
	}

	return key + "=" + strings.Join(properties, ","), true
}

func (node yamlStringNode) terraformValue(value any, valueType tftypes.Type) (tftypes.Value, error) {
	var decoded reflect.Value = reflect.New(node.kind)

	content, err := yaml.Marshal(value)

	if err == nil {
		err = yaml.Unmarshal(content, decoded.Interface())
	}

	if err != nil {
		return tftypes.Value{}, err
	}

	return tftypes.NewValue(valueType, decoded.Elem().String()), nil
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
	var validators []validator.Int64

	if minimum, maximum, bounded := integerRange(field.kind); bounded {
		validators = append(validators, int64validator.Between(minimum, maximum))
	}

	return schema.Int64Attribute{
		Description: field.description,
		Required:    !field.optional,
		Optional:    field.optional,
		Validators:  validators,
	}
}

// integerRange returns the range of an integer kind narrower than Int64, reporting false otherwise.
func integerRange(kind reflect.Type) (int64, int64, bool) {
	for kind.Kind() == reflect.Pointer {
		kind = kind.Elem()
	}

	switch kind.Kind() {
	case reflect.Int8, reflect.Int16, reflect.Int32:
		var bits int = kind.Bits()

		return -1 << (bits - 1), 1<<(bits-1) - 1, true
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return 0, 1<<kind.Bits() - 1, true
	case reflect.Uint, reflect.Uint64:
		return 0, math.MaxInt64, true
	}

	return 0, 0, false
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

// bigFloat converts a YAML number, whose whole floats decode as ints, into a Terraform number.
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
