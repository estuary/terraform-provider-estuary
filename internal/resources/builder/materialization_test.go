package builder

import (
	"context"
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"gopkg.in/yaml.v3"
)

type testTopic struct {
	TopicName string `yaml:"topic_name"`
}

type testDekafEndpoint struct {
	Variant string     `yaml:"variant"`
	Config  testConfig `yaml:"config"`
}

type testDekafEndpointSpec struct {
	Dekaf testDekafEndpoint `yaml:"dekaf"`
}

type testDekafSpec = materialization.Spec[testDekafEndpointSpec, testTopic]

type testDekafDocument struct {
	Name     string                               `yaml:"name"`
	Variant  string                               `yaml:"variant"`
	Config   testConfig                           `yaml:"config"`
	Source   *materialization.Source              `yaml:"source,omitempty"`
	Bindings []materialization.Binding[testTopic] `yaml:"bindings"`
}

func (document testDekafDocument) Spec() (string, testDekafSpec) {
	return document.Name, testDekafSpec{
		Endpoint: testDekafEndpointSpec{Dekaf: testDekafEndpoint{Variant: document.Variant, Config: document.Config}},
		Source:   document.Source,
		Bindings: document.Bindings,
	}
}

func (testDekafDocument) FromSpec(name string, spec testDekafSpec) testDekafDocument {
	return testDekafDocument{
		Name:     name,
		Variant:  spec.Endpoint.Dekaf.Variant,
		Config:   spec.Endpoint.Dekaf.Config,
		Source:   spec.Source,
		Bindings: spec.Bindings,
	}
}

func TestMaterializationCodecSchema(test *testing.T) {
	var ctx context.Context = context.Background()

	codec, err := NewMaterializationCodec[testConfig, testResource](testConnector)

	if err != nil {
		test.Fatal(err)
	}

	if diagnostics := codec.Schema().ValidateImplementation(ctx); diagnostics.HasError() {
		test.Fatalf("invalid schema: %v", diagnostics)
	}

	expectations := map[string]struct {
		path     path.Path
		kind     any
		required bool
	}{
		"name":                       {path.Root("name"), schema.StringAttribute{}, true},
		"image":                      {path.Root("image"), schema.StringAttribute{}, false},
		"config":                     {path.Root("config"), schema.SingleNestedAttribute{}, true},
		"source":                     {path.Root("source"), schema.SingleNestedAttribute{}, false},
		"source capture":             {path.Root("source").AtName("capture"), schema.StringAttribute{}, false},
		"source fields_recommended":  {path.Root("source").AtName("fields_recommended"), schema.Int64Attribute{}, true},
		"bindings":                   {path.Root("bindings"), schema.ListNestedAttribute{}, true},
		"binding source":             {path.Root("bindings").AtListIndex(0).AtName("source"), schema.StringAttribute{}, true},
		"binding fields":             {path.Root("bindings").AtListIndex(0).AtName("fields"), schema.SingleNestedAttribute{}, false},
		"binding fields exclude":     {path.Root("bindings").AtListIndex(0).AtName("fields").AtName("exclude"), schema.ListAttribute{}, false},
		"binding fields recommended": {path.Root("bindings").AtListIndex(0).AtName("fields").AtName("recommended"), schema.BoolAttribute{}, true},
		"binding fields require":     {path.Root("bindings").AtListIndex(0).AtName("fields").AtName("require"), schema.SetAttribute{}, false},
		"binding backfill":           {path.Root("bindings").AtListIndex(0).AtName("backfill"), schema.Int64Attribute{}, false},
		"binding name":               {path.Root("bindings").AtListIndex(0).AtName("name"), schema.StringAttribute{}, false},
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

		if attribute.IsRequired() != expected.required {
			test.Errorf("%s: required %v, want %v", name, attribute.IsRequired(), expected.required)
		}
	}
}

func TestMaterializationCodecRoundTrip(test *testing.T) {
	var capture string = "acmeCo/source-postgres"

	codec, err := NewMaterializationCodec[testConfig, testResource](testConnector)

	if err != nil {
		test.Fatal(err)
	}

	specs := map[string]MaterializationSpec[testConfig, testResource]{
		"acmeCo/materialize-full": {
			Endpoint: endpoint.ConnectorSpec[testConfig]{
				Connector: endpoint.Connector[testConfig]{
					Image:  testConnector.Image,
					Config: testConfig{Host: "warehouse.example.com", Password: "<secret>", Labels: map[string]string{"team": "data"}},
				},
			},
			Source: &materialization.Source{
				Capture:           &capture,
				TargetNaming:      "withSchema",
				DeltaUpdates:      true,
				FieldsRecommended: 2,
			},
			Bindings: []materialization.Binding[testResource]{
				{Resource: testResource{Table: "orders", Priority: 3}, Source: "acmeCo/orders", Fields: &materialization.BindingFields{Recommended: true, Exclude: []string{"_meta"}, Require: map[string]struct{}{"flow_document": {}, "_meta/op": {}}}},
				{Resource: testResource{Table: "customers"}, Source: "acmeCo/customers", Backfill: 2},
			},
		},
		"acmeCo/materialize-null-capture": {
			Endpoint: endpoint.ConnectorSpec[testConfig]{
				Connector: endpoint.Connector[testConfig]{
					Image:  testConnector.Image,
					Config: testConfig{Host: "warehouse.example.com"},
				},
			},
			Source: &materialization.Source{
				TargetNaming: "prefixSchema",
			},
			Bindings: []materialization.Binding[testResource]{
				{Resource: testResource{Table: "orders"}, Source: "acmeCo/orders"},
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

			for index, binding := range spec.Bindings {
				bindingName, err := attributeValue(value, tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(index).WithAttributeName("name"))

				if err != nil || !bindingName.Equal(tftypes.NewValue(tftypes.String, binding.Source)) {
					test.Errorf("bindings[%d].name = %v (%v), want the source %s", index, bindingName, err, binding.Source)
				}
			}
		})
	}
}

func TestMaterializationFromDocument(test *testing.T) {
	var ctx context.Context = context.Background()
	var connector Connector = Connector{TypeName: "materialization_test_dekaf"}
	var spec testDekafSpec = testDekafSpec{
		Endpoint: testDekafEndpointSpec{Dekaf: testDekafEndpoint{Variant: "tinybird", Config: testConfig{Host: "dekaf.example.com", Password: "<token>"}}},
		Bindings: []materialization.Binding[testTopic]{
			{Resource: testTopic{TopicName: "orders"}, Source: "acmeCo/orders"},
		},
	}

	codec, err := NewCodec[testDekafDocument](connector)

	if err != nil {
		test.Fatal(err)
	}

	if _, diagnostics := codec.Schema().AttributeAtPath(ctx, path.Root("image")); !diagnostics.HasError() {
		test.Error("schema has an image attribute, want none for a document without image")
	}

	value, err := codec.Value("acmeCo/dekaf", spec)

	if err != nil {
		test.Fatal(err)
	}

	name, roundTrip, err := codec.Spec(value)

	if err != nil {
		test.Fatal(err)
	}

	if name != "acmeCo/dekaf" || !reflect.DeepEqual(roundTrip, spec) {
		test.Errorf("round trip = %s %+v, want acmeCo/dekaf %+v", name, roundTrip, spec)
	}

	var metadataResp resource.MetadataResponse

	NewMaterializationFromDocument[testDekafDocument](connector)().Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "estuary"}, &metadataResp)

	if metadataResp.TypeName != "estuary_materialization_test_dekaf" {
		test.Errorf("type name = %q, want estuary_materialization_test_dekaf", metadataResp.TypeName)
	}
}

func TestNewCodecRequiresConnectorImage(test *testing.T) {
	if _, err := NewMaterializationCodec[testConfig, testResource](Connector{TypeName: "materialization_test"}); err == nil {
		test.Error("NewMaterializationCodec accepted a connector without a default image")
	}
}
