package builder

import (
	"context"
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"gopkg.in/yaml.v3"
)

type testConfig struct {
	Host     string            `yaml:"host" description:"Database host."`
	NodeID   uint32            `yaml:"node_id,omitempty"`
	Port     int               `yaml:"port,omitempty"`
	Ratio    float64           `yaml:"ratio,omitempty"`
	ReadOnly bool              `yaml:"readOnly,omitempty"`
	Password string            `yaml:"password,omitempty"`
	Schemas  []string          `yaml:"schemas,omitempty"`
	Tunnel   *testTunnel       `yaml:"tunnel,omitempty"`
	Labels   map[string]string `yaml:"labels,omitempty"`
}

type testTunnel struct {
	SSHEndpoint string `yaml:"sshEndpoint"`
}

type testResource struct {
	Table    string `yaml:"table"`
	Priority int    `yaml:"priority,omitempty"`
}

func (resource testResource) ResourcePath() []string {
	return []string{resource.Table}
}

var testConnector Connector = Connector{
	TypeName: "capture_test",
	Image:    "ghcr.io/estuary/source-test:v1",
}

func TestCaptureCodecSchema(test *testing.T) {
	var ctx context.Context = context.Background()

	codec, err := newCodec[captureDocument[testConfig, testResource]](testConnector, captureType[testResource]())

	if err != nil {
		test.Fatal(err)
	}

	if diagnostics := codec.Schema().ValidateImplementation(ctx); diagnostics.HasError() {
		test.Fatalf("invalid schema: %v", diagnostics)
	}

	expectations := map[string]struct {
		path      path.Path
		kind      any
		required  bool
		sensitive bool
		writeOnly bool
	}{
		"name":             {path.Root("name"), schema.StringAttribute{}, true, false, false},
		"image":            {path.Root("image"), schema.StringAttribute{}, false, false, false},
		"config":           {path.Root("config"), schema.SingleNestedAttribute{}, true, false, false},
		"host":             {path.Root("config").AtName("host"), schema.StringAttribute{}, true, false, false},
		"port":             {path.Root("config").AtName("port"), schema.Int64Attribute{}, false, false, false},
		"node_id":          {path.Root("config").AtName("node_id"), schema.Int64Attribute{}, false, false, false},
		"ratio":            {path.Root("config").AtName("ratio"), schema.Float64Attribute{}, false, false, false},
		"read_only":        {path.Root("config").AtName("read_only"), schema.BoolAttribute{}, false, false, false},
		"password":         {path.Root("config").AtName("password"), schema.StringAttribute{}, false, true, true},
		"password_hash":    {path.Root("config").AtName("password_hash"), schema.StringAttribute{}, false, true, false},
		"schemas":          {path.Root("config").AtName("schemas"), schema.ListAttribute{}, false, false, false},
		"tunnel":           {path.Root("config").AtName("tunnel"), schema.SingleNestedAttribute{}, false, false, false},
		"labels":           {path.Root("config").AtName("labels"), schema.MapAttribute{}, false, false, false},
		"bindings":         {path.Root("bindings"), schema.ListNestedAttribute{}, true, false, false},
		"auto_discover":    {path.Root("auto_discover"), schema.SingleNestedAttribute{}, false, false, false},
		"binding resource": {path.Root("bindings").AtListIndex(0).AtName("resource"), schema.SingleNestedAttribute{}, true, false, false},
		"binding target":   {path.Root("bindings").AtListIndex(0).AtName("target"), schema.StringAttribute{}, false, false, false},
		"binding name":     {path.Root("bindings").AtListIndex(0).AtName("name"), schema.StringAttribute{}, false, false, false},
	}

	for name, expected := range expectations {
		attribute, diagnostics := codec.Schema().AttributeAtPath(ctx, expected.path)

		if diagnostics.HasError() {
			test.Errorf("%s: %v", name, diagnostics)
			continue
		}

		if reflect.TypeOf(attribute) != reflect.TypeOf(expected.kind) {
			test.Errorf("%s: attribute = %T, want %T", name, attribute, expected.kind)
		}

		if attribute.IsRequired() != expected.required || attribute.IsSensitive() != expected.sensitive || attribute.IsWriteOnly() != expected.writeOnly {
			test.Errorf("%s: required %v, sensitive %v, write-only %v, want %v, %v, %v", name, attribute.IsRequired(), attribute.IsSensitive(), attribute.IsWriteOnly(), expected.required, expected.sensitive, expected.writeOnly)
		}
	}

	host, _ := codec.Schema().AttributeAtPath(ctx, path.Root("config").AtName("host"))
	image, _ := codec.Schema().AttributeAtPath(ctx, path.Root("image"))

	if host.GetDescription() != "Database host." {
		test.Errorf("host description = %q, want the description tag", host.GetDescription())
	}

	if target, _ := codec.Schema().AttributeAtPath(ctx, path.Root("bindings").AtListIndex(0).AtName("target")); !target.IsComputed() {
		test.Error("binding target is not computed, want the discovery name as default")
	}

	if bindingName, _ := codec.Schema().AttributeAtPath(ctx, path.Root("bindings").AtListIndex(0).AtName("name")); !bindingName.IsComputed() || bindingName.IsOptional() {
		test.Error("binding name is not computed only, want the provider to set it")
	}

	if hash, _ := codec.Schema().AttributeAtPath(ctx, path.Root("config").AtName("password_hash")); !hash.IsComputed() {
		test.Error("password_hash is not computed, want the provider to set it")
	}

	if _, diagnostics := codec.Schema().AttributeAtPath(ctx, path.Root("config").AtName("host_hash")); !diagnostics.HasError() {
		test.Error("host has a hash, want hashes only beside write-only attributes")
	}

	if !image.IsComputed() {
		test.Error("image is not computed, want the connector image as default")
	}
}

func TestDocumentEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[captureDocument[struct{}, struct{}]](test, backfillStrategies)

	spectest.AssertEnumTags[materializationDocument[struct{}, struct{}]](test, materialization.OnIncompatibleSchemaChanges, materialization.TargetNamingStrategies)
}

func TestCaptureCodecImageAllowsOnlyTags(test *testing.T) {
	var ctx context.Context = context.Background()

	codec, err := newCodec[captureDocument[testConfig, testResource]](testConnector, captureType[testResource]())

	if err != nil {
		test.Fatal(err)
	}

	attribute, _ := codec.Schema().Attributes[imageAttribute].(schema.StringAttribute)

	expectations := map[string]bool{
		"ghcr.io/estuary/source-test:v1":          true,
		"ghcr.io/estuary/source-test:dev":         true,
		"ghcr.io/estuary/source-test":             false,
		"ghcr.io/estuary/source-test:":            false,
		"ghcr.io/estuary/source-other:v1":         false,
		"ghcr.io/estuary/source-test-fork:v1":     false,
		"attacker.io/estuary/source-test:v1":      false,
		"ghcr.io/estuary/source-test@sha256:abcd": false,
		"ghcr.io/estuary/source-test:v1/extra":    false,
	}

	for image, valid := range expectations {
		var validateResp validator.StringResponse

		for _, check := range attribute.Validators {
			check.ValidateString(ctx, validator.StringRequest{Path: path.Root(imageAttribute), ConfigValue: types.StringValue(image)}, &validateResp)
		}

		if validateResp.Diagnostics.HasError() == valid {
			test.Errorf("image %q: diagnostics %v, want valid %v", image, validateResp.Diagnostics, valid)
		}
	}

	if _, err := newCodec[captureDocument[testConfig, testResource]](Connector{TypeName: "capture_test", Image: "ghcr.io/estuary/source-test"}, captureType[testResource]()); err == nil {
		test.Error("newCodec accepted a default image without a tag")
	}
}

func TestCaptureCodecRoundTrip(test *testing.T) {
	codec, err := newCodec[captureDocument[testConfig, testResource]](testConnector, captureType[testResource]())

	if err != nil {
		test.Fatal(err)
	}

	specs := map[string]CaptureSpec[testConfig, testResource]{
		"acmeCo/source-full": {
			AutoDiscover: &capture.AutoDiscover{
				AddNewBindings: true,
			},
			Endpoint: endpoint.ConnectorSpec[testConfig]{
				Connector: endpoint.Connector[testConfig]{
					Image: testConnector.Image,
					Config: testConfig{
						Host:     "db.example.com",
						NodeID:   1234567,
						Port:     5432,
						Ratio:    2,
						ReadOnly: true,
						Password: "<secret>",
						Schemas:  []string{"public", "sales"},
						Tunnel:   &testTunnel{SSHEndpoint: "ssh://bastion:22"},
						Labels:   map[string]string{"team": "data", "tier": "gold"},
					},
				},
			},
			Bindings: []capture.Binding[testResource]{
				{Resource: testResource{Table: "orders", Priority: -1}, Target: "acmeCo/orders"},
				{Resource: testResource{Table: "logs"}, Target: "acmeCo/logs", Disable: true},
			},
			Shards: catalog.Shards{
				"disable":        true,
				"logLevel":       "debug",
				"maxTxnDuration": "4m",
				"hotStandbys":    1,
				"flags":          map[string]any{"enable_feature": true, "ratio": 0.5},
				"notes":          []any{"first", 2},
			},
		},
		"acmeCo/source-minimal": {
			Endpoint: endpoint.ConnectorSpec[testConfig]{
				Connector: endpoint.Connector[testConfig]{
					Image:  testConnector.Image,
					Config: testConfig{Host: "db.example.com"},
				},
			},
		},
	}

	for name, spec := range specs {
		test.Run(name, func(test *testing.T) {
			value, err := codec.Value(name, spec)

			if err != nil {
				test.Fatalf("converting spec to Terraform: %v", err)
			}

			roundTripName, roundTrip, err := codec.Spec(value)

			if err != nil {
				test.Fatalf("converting Terraform to spec: %v", err)
			}

			published, err := yaml.Marshal(spec)

			if err != nil {
				test.Fatal(err)
			}

			roundTripPublished, err := yaml.Marshal(roundTrip)

			if err != nil {
				test.Fatal(err)
			}

			if roundTripName != name || string(roundTripPublished) != string(published) {
				test.Errorf("round trip = %s\n%s\nwant %s\n%s", roundTripName, roundTripPublished, name, published)
			}
		})
	}
}

func TestCaptureCodecRejectsUnknownValues(test *testing.T) {
	var attributes map[string]tftypes.Value
	var spec CaptureSpec[testConfig, testResource] = CaptureSpec[testConfig, testResource]{
		Endpoint: endpoint.ConnectorSpec[testConfig]{
			Connector: endpoint.Connector[testConfig]{
				Image:  testConnector.Image,
				Config: testConfig{Host: "db.example.com"},
			},
		},
	}

	codec, err := newCodec[captureDocument[testConfig, testResource]](testConnector, captureType[testResource]())

	if err != nil {
		test.Fatal(err)
	}

	value, err := codec.Value("acmeCo/source-test", spec)

	if err != nil {
		test.Fatal(err)
	}

	if err := value.As(&attributes); err != nil {
		test.Fatal(err)
	}

	attributes["name"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)

	if _, _, err := codec.Spec(tftypes.NewValue(value.Type(), attributes)); err == nil {
		test.Error("Spec accepted an unknown name, want an error")
	}
}
