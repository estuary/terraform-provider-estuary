package translator

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
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

func TestObjectRejectsCredentialsInForbiddingFields(test *testing.T) {
	var ctx context.Context = context.Background()

	type fields struct {
		Address string `yaml:"address" credentials:"forbidden"`
	}

	object, err := NewObject(reflect.TypeFor[fields]())

	if err != nil {
		test.Fatal(err)
	}

	attribute, _ := object.Attributes()["address"].(schema.StringAttribute)

	expectations := map[string]bool{
		"mongodb+srv://cluster0.123456789.mongodb.net":                               true,
		"mongodb://123.4.567.89:27017/?authSource=admin&tls=true":                    true,
		"mongodb://123.4.567.89:27017/?authMechanismProperties=SERVICE_NAME:mongodb": true,
		"123.4.567.89:27017": true,
		"mongodb://flow_capture:secret@123.4.567.89:27017":                                           false,
		"mongodb+srv://flow_capture@cluster0.123456789.mongodb.net":                                  false,
		"flow_capture:secret@123.4.567.89:27017":                                                     false,
		"mongodb:/flow_capture:secret@123.4.567.89:27017":                                            false,
		"mongodb://flow_capture:sec/ret@123.4.567.89:27017":                                          false,
		"mongodb://flow_capture:sec?ret@123.4.567.89:27017":                                          false,
		"mongodb://flow_capture:sec#ret@123.4.567.89:27017":                                          false,
		"mongodb://123.4.567.89:27017/db#user@fragment":                                              false,
		"mongodb://123.4.567.89:27017/?tlsCertificateKeyFilePassword=x":                              false,
		"mongodb://123.4.567.89:27017/?authSource=admin&password=secret":                             false,
		"mongodb://123.4.567.89:27017/?authSource=admin;tlsCertificateKeyFilePassword=x":             false,
		"mongodb://123.4.567.89:27017/?PASSWORD=x":                                                   false,
		"mongodb://123.4.567.89:27017/?%70assword=x":                                                 false,
		"mongodb://123.4.567.89:27017/?authMechanismProperties=AWS_SESSION_TOKEN:abc":                false,
		"mongodb://123.4.567.89:27017/?authMechanismProperties=SERVICE_NAME:a,AWS_SESSION_TOKEN:abc": false,
		"mongodb://***@123.4.567.89:27017":                                                           false,
		"mongodb://123.4.567.89:27017/?password=***":                                                 false,
		"flow_capture:secret@123.4.567.89:27017/?appName=http://x":                                   false,
		"https://123.4.567.89/api/orders?code=abc":                                                   false,
		"https://123.4.567.89/workflows/run?sv=1.0&sig=abc":                                          false,
		"https://123.4.567.89/?APIKEY=abc":                                                           false,
		"https://123.4.567.89/object?X-Amz-Expires=60&X-Amz-Signature=abc":                           false,
		"https://123.4.567.89/object?X-Goog-Signature=abc":                                           false,
		"https://123.4.567.89/?signature=abc":                                                        false,
		"https://123.4.567.89/?country_code=pt&design=a":                                             true,
	}

	for address, valid := range expectations {
		var validateResp validator.StringResponse

		for _, check := range attribute.Validators {
			check.ValidateString(ctx, validator.StringRequest{Path: path.Root("address"), ConfigValue: types.StringValue(address)}, &validateResp)
		}

		if validateResp.Diagnostics.HasError() == valid {
			test.Errorf("address %q: diagnostics %v, want valid %v", address, validateResp.Diagnostics, valid)
		}

		for _, diagnostic := range validateResp.Diagnostics {
			if strings.Contains(diagnostic.Detail(), "secret") || strings.Contains(diagnostic.Summary(), "secret") {
				test.Errorf("address %q: diagnostic %q echoes the credentials", address, diagnostic.Detail())
			}
		}
	}
}

func TestRedactCredentials(test *testing.T) {
	expectations := map[string]string{
		"mongodb+srv://cluster0.123456789.mongodb.net?authSource=admin":                                 "mongodb+srv://cluster0.123456789.mongodb.net?authSource=admin",
		"mongodb://flow_capture:secret@123.4.567.89:27017":                                              "mongodb://***@123.4.567.89:27017",
		"mongodb://flow_capture:sec/ret@123.4.567.89:27017/db":                                          "mongodb://***@123.4.567.89:27017/db",
		"flow_capture:secret@123.4.567.89:27017":                                                        "***@123.4.567.89:27017",
		"mongodb://***@123.4.567.89:27017":                                                              "mongodb://***@123.4.567.89:27017",
		"mongodb://123.4.567.89:27017/?authSource=admin&password=secret#frag":                           "mongodb://123.4.567.89:27017/?authSource=admin&password=***",
		"mongodb://123.4.567.89:27017/?tls=true;tlsCertificateKeyFilePassword=secret":                   "mongodb://123.4.567.89:27017/?tls=true;tlsCertificateKeyFilePassword=***",
		"mongodb://123.4.567.89:27017/?authMechanismProperties=SERVICE_NAME:a,AWS_SESSION_TOKEN:secret": "mongodb://123.4.567.89:27017/?authMechanismProperties=SERVICE_NAME:a,AWS_SESSION_TOKEN:***",
		"flow_capture:secret@123.4.567.89:27017/?appName=http://x":                                      "***@123.4.567.89:27017/?appName=http://x",
		"flow_capture:sec://ret@123.4.567.89:27017":                                                     "***@123.4.567.89:27017",
		"mongodb://123.4.567.89:27017/?password=sec#ret":                                                "mongodb://123.4.567.89:27017/?password=***",
		"mongodb://123.4.567.89:27017/?tls=true#fragment":                                               "mongodb://123.4.567.89:27017/?tls=true#fragment",
		"https://123.4.567.89/api/orders?code=abc&page=2":                                               "https://123.4.567.89/api/orders?code=***&page=2",
		"https://123.4.567.89/workflows/run?sv=1.0&sig=abc":                                             "https://123.4.567.89/workflows/run?sv=1.0&sig=***",
		"https://123.4.567.89/?apiKey=abc":                                                              "https://123.4.567.89/?apiKey=***",
		"https://123.4.567.89/object?X-Amz-Expires=60&X-Amz-Signature=abc":                              "https://123.4.567.89/object?X-Amz-Expires=60&X-Amz-Signature=***",
		"https://123.4.567.89/?country_code=pt":                                                         "https://123.4.567.89/?country_code=pt",
	}

	for value, want := range expectations {
		if got := RedactCredentials(value, catalog.CredentialsForbidden); got != want {
			test.Errorf("RedactCredentials(%q) = %q, want %q", value, got, want)
		}

		if again := RedactCredentials(want, catalog.CredentialsForbidden); again != want {
			test.Errorf("RedactCredentials(%q) = %q, want it stable", want, again)
		}
	}
}

func TestObjectRejectsPasswordsInPasswordFields(test *testing.T) {
	var ctx context.Context = context.Background()

	type fields struct {
		SSHEndpoint string `yaml:"sshEndpoint" credentials:"password"`
	}

	object, err := NewObject(reflect.TypeFor[fields]())

	if err != nil {
		test.Fatal(err)
	}

	attribute, _ := object.Attributes()["ssh_endpoint"].(schema.StringAttribute)

	expectations := map[string]bool{
		"ssh://ec2-user@ec2-123-4-567-89.compute-1.amazonaws.com:22": true,
		"ssh://123.4.567.89":                        true,
		"ec2-user@123.4.567.89:22":                  true,
		"ssh://ec2-user:secret@123.4.567.89:22":     false,
		"ssh://ec2-user:sec@ret@123.4.567.89:22":    false,
		"ec2-user:secret@123.4.567.89:22":           false,
		"ssh://ec2-user:***@123.4.567.89:22":        false,
		"ssh://ec2-user@123.4.567.89:22?password=x": false,
	}

	for endpoint, valid := range expectations {
		var validateResp validator.StringResponse

		for _, check := range attribute.Validators {
			check.ValidateString(ctx, validator.StringRequest{Path: path.Root("ssh_endpoint"), ConfigValue: types.StringValue(endpoint)}, &validateResp)
		}

		if validateResp.Diagnostics.HasError() == valid {
			test.Errorf("endpoint %q: diagnostics %v, want valid %v", endpoint, validateResp.Diagnostics, valid)
		}

		for _, diagnostic := range validateResp.Diagnostics {
			if strings.Contains(diagnostic.Detail(), "secret") || strings.Contains(diagnostic.Summary(), "secret") {
				test.Errorf("endpoint %q: diagnostic %q echoes the password", endpoint, diagnostic.Detail())
			}
		}
	}

	if got := AttributeCredentials(attribute); got != catalog.CredentialsPassword {
		test.Errorf("AttributeCredentials = %q, want %q", got, catalog.CredentialsPassword)
	}
}

func TestRedactPassword(test *testing.T) {
	expectations := map[string]string{
		"ssh://ec2-user@123.4.567.89:22":         "ssh://ec2-user@123.4.567.89:22",
		"ssh://ec2-user:secret@123.4.567.89:22":  "ssh://ec2-user:***@123.4.567.89:22",
		"ssh://ec2-user:sec@ret@123.4.567.89:22": "ssh://ec2-user:***@123.4.567.89:22",
		"ssh://ec2-user:sec:ret@123.4.567.89":    "ssh://ec2-user:***@123.4.567.89",
		"ec2-user:secret@123.4.567.89:22":        "ec2-user:***@123.4.567.89:22",
		"ssh://123.4.567.89:22":                  "ssh://123.4.567.89:22",
	}

	for value, want := range expectations {
		if got := RedactCredentials(value, catalog.CredentialsPassword); got != want {
			test.Errorf("RedactCredentials(%q) = %q, want %q", value, got, want)
		}

		if again := RedactCredentials(want, catalog.CredentialsPassword); again != want {
			test.Errorf("RedactCredentials(%q) = %q, want it stable", want, again)
		}
	}
}

func TestObjectRejectsEmptySecrets(test *testing.T) {
	var ctx context.Context = context.Background()

	type fields struct {
		Password string `yaml:"password,omitempty" secret:"true"`
	}

	object, err := NewObject(reflect.TypeFor[fields]())

	if err != nil {
		test.Fatal(err)
	}

	attribute, _ := object.Attributes()["password"].(schema.StringAttribute)

	expectations := map[string]struct {
		value types.String
		valid bool
	}{
		"a value":         {types.StringValue("secret"), true},
		"left out":        {types.StringNull(), true},
		"known at apply":  {types.StringUnknown(), true},
		"an empty string": {types.StringValue(""), false},
	}

	for name, expected := range expectations {
		var validateResp validator.StringResponse

		for _, check := range attribute.Validators {
			check.ValidateString(ctx, validator.StringRequest{Path: path.Root("password"), ConfigValue: expected.value}, &validateResp)
		}

		if validateResp.Diagnostics.HasError() == expected.valid {
			test.Errorf("%s: diagnostics %v, want valid %v", name, validateResp.Diagnostics, expected.valid)
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
