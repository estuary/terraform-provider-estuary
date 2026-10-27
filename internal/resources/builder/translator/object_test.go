package translator

import (
	"context"
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestObjectAttributesHashWriteOnlySecrets(test *testing.T) {
	type tunnel struct {
		PrivateKey string `yaml:"privateKey"`
	}

	type config struct {
		Host     string   `yaml:"host"`
		Password string   `yaml:"password"`
		Tunnel   *tunnel  `yaml:"tunnel,omitempty"`
		Tokens   []string `yaml:"tokens,omitempty"`
	}

	object, err := NewObject(reflect.TypeFor[config]())

	if err != nil {
		test.Fatal(err)
	}

	var attributes map[string]schema.Attribute = object.Attributes()

	if hash, found := attributes["password_hash"]; !found || !hash.IsComputed() || !hash.IsSensitive() || hash.IsWriteOnly() {
		test.Errorf("password_hash = %v, want a computed sensitive hash beside the write-only password", hash)
	}

	for _, attribute := range []string{"host_hash", "tunnel_hash", "tokens_hash"} {
		if _, found := attributes[attribute]; found {
			test.Errorf("attribute %s is set, want hashes only beside write-only attributes", attribute)
		}
	}

	nested, _ := attributes["tunnel"].(schema.SingleNestedAttribute)

	if _, found := nested.Attributes["private_key_hash"]; !found {
		test.Error("tunnel has no private_key_hash, want hashes beside nested write-only attributes")
	}
}

func TestNewObjectRejectsHashCollisions(test *testing.T) {
	type collision struct {
		Password     string `yaml:"password"`
		PasswordHash string `yaml:"password_hash"`
	}

	if _, err := NewObject(reflect.TypeFor[collision]()); err == nil {
		test.Error("NewObject accepted a password_hash field beside the write-only password")
	}
}

func TestObjectInlinesEmbeddedStructs(test *testing.T) {
	type options struct {
		Priority int  `yaml:"priority,omitempty"`
		Disable  bool `yaml:"disable,omitempty"`
	}

	type binding struct {
		Source  string `yaml:"source"`
		options `yaml:",inline"`
	}

	object, err := NewObject(reflect.TypeFor[binding]())

	if err != nil {
		test.Fatal(err)
	}

	var objectType tftypes.Type = tftypes.Object{AttributeTypes: map[string]tftypes.Type{"source": tftypes.String, "priority": tftypes.Number, "disable": tftypes.Bool}}

	value, err := object.TerraformValue(map[string]any{"source": "acmeCo/orders", "priority": 3}, objectType)

	if err != nil {
		test.Fatal(err)
	}

	converted, err := object.SpecValue(value)

	if err != nil || !reflect.DeepEqual(converted, map[string]any{"source": "acmeCo/orders", "priority": int64(3)}) {
		test.Errorf("spec value = %v (%v), want the inlined priority beside the source", converted, err)
	}

	if _, found := object.Attributes()["priority"]; !found {
		test.Error("attributes have no priority, want the inlined fields flattened")
	}
}

func TestObjectReadsYAMLStringTypesFromOtherScalars(test *testing.T) {
	type fields struct {
		Recommended materialization.FieldDepth `yaml:"recommended"`
	}

	object, err := NewObject(reflect.TypeFor[fields]())

	if err != nil {
		test.Fatal(err)
	}

	var objectType tftypes.Type = tftypes.Object{AttributeTypes: map[string]tftypes.Type{"recommended": tftypes.String}}

	for written, want := range map[any]string{true: "unlimited", 2: "2", false: "0"} {
		value, err := object.TerraformValue(map[string]any{"recommended": written}, objectType)

		if err != nil || !value.Equal(tftypes.NewValue(objectType, map[string]tftypes.Value{"recommended": tftypes.NewValue(tftypes.String, want)})) {
			test.Errorf("recommended %v = %v (%v), want %q", written, value, err, want)
		}
	}
}

func TestObjectValidatesYAMLStringTypes(test *testing.T) {
	var ctx context.Context = context.Background()

	type fields struct {
		Recommended materialization.FieldDepth `yaml:"recommended"`
	}

	object, err := NewObject(reflect.TypeFor[fields]())

	if err != nil {
		test.Fatal(err)
	}

	attribute, _ := object.Attributes()["recommended"].(schema.StringAttribute)

	for value, valid := range map[string]bool{"1": true, "unlimited": true, "-1": false, "deep": false} {
		var validateResp validator.StringResponse

		for _, check := range attribute.Validators {
			check.ValidateString(ctx, validator.StringRequest{Path: path.Root("recommended"), ConfigValue: types.StringValue(value)}, &validateResp)
		}

		if validateResp.Diagnostics.HasError() == valid {
			test.Errorf("recommended %q: diagnostics %v, want valid %v", value, validateResp.Diagnostics, valid)
		}
	}
}
