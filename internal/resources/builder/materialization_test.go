package builder

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
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
	Name                        string     `yaml:"name"`
	Variant                     string     `yaml:"variant"`
	Config                      testConfig `yaml:"config"`
	materialization.SpecOptions `yaml:",inline"`
	Bindings                    []MaterializationBinding[testTopic] `yaml:"bindings"`
}

func (document testDekafDocument) Spec() (string, testDekafSpec) {
	return document.Name, testDekafSpec{
		Endpoint:    testDekafEndpointSpec{Dekaf: testDekafEndpoint{Variant: document.Variant, Config: document.Config}},
		SpecOptions: document.SpecOptions,
		Bindings:    MaterializationBindingsSpec(document.Bindings),
	}
}

func (testDekafDocument) FromSpec(name string, spec testDekafSpec) testDekafDocument {
	return testDekafDocument{
		Name:        name,
		Variant:     spec.Endpoint.Dekaf.Variant,
		Config:      spec.Endpoint.Dekaf.Config,
		SpecOptions: spec.SpecOptions,
		Bindings:    NewMaterializationBindings(spec.Bindings),
	}
}

func TestMaterializationCodecSchema(test *testing.T) {
	var ctx context.Context = context.Background()

	codec, err := newCodec[materializationDocument[testConfig, testResource]](testConnector, materializationType)

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
		"source fields_recommended":  {path.Root("source").AtName("fields_recommended"), schema.StringAttribute{}, false},
		"target_naming strategy":     {path.Root("target_naming").AtName("strategy"), schema.StringAttribute{}, true},
		"incompatible schema change": {path.Root("on_incompatible_schema_change"), schema.StringAttribute{}, false},
		"bindings":                   {path.Root("bindings"), schema.ListNestedAttribute{}, true},
		"binding source":             {path.Root("bindings").AtListIndex(0).AtName("source"), schema.StringAttribute{}, true},
		"binding fields":             {path.Root("bindings").AtListIndex(0).AtName("fields"), schema.SingleNestedAttribute{}, false},
		"binding fields exclude":     {path.Root("bindings").AtListIndex(0).AtName("fields").AtName("exclude"), schema.ListAttribute{}, false},
		"binding fields recommended": {path.Root("bindings").AtListIndex(0).AtName("fields").AtName("recommended"), schema.StringAttribute{}, true},
		"binding fields group_by":    {path.Root("bindings").AtListIndex(0).AtName("fields").AtName("group_by"), schema.ListAttribute{}, false},
		"binding not_before":         {path.Root("bindings").AtListIndex(0).AtName("not_before"), schema.StringAttribute{}, false},
		"binding disable":            {path.Root("bindings").AtListIndex(0).AtName("disable"), schema.BoolAttribute{}, false},
		"binding priority":           {path.Root("bindings").AtListIndex(0).AtName("priority"), schema.Int64Attribute{}, false},
		"binding fields require":     {path.Root("bindings").AtListIndex(0).AtName("fields").AtName("require"), schema.SetAttribute{}, false},
		"binding backfill":           {path.Root("bindings").AtListIndex(0).AtName("backfill"), schema.SingleNestedAttribute{}, false},
		"binding backfill trigger":   {path.Root("bindings").AtListIndex(0).AtName("backfill").AtName("trigger"), schema.Int64Attribute{}, false},
		"binding name":               {path.Root("bindings").AtListIndex(0).AtName("name"), schema.StringAttribute{}, false},
		"backfill":                   {path.Root("backfill"), schema.SingleNestedAttribute{}, false},
		"backfill trigger":           {path.Root("backfill").AtName("trigger"), schema.Int64Attribute{}, false},
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

	codec, err := newCodec[materializationDocument[testConfig, testResource]](testConnector, materializationType)

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
			SpecOptions: materialization.SpecOptions{
				Source:                     &materialization.Source{Capture: &capture, DeltaUpdates: true, FieldsRecommended: "2"},
				TargetNaming:               &materialization.TargetNaming{Strategy: materialization.TargetNamingPrefixTableNames, Schema: "analytics", SkipCommonDefaults: true, TableTemplate: "raw_{{table}}"},
				OnIncompatibleSchemaChange: materialization.OnIncompatibleSchemaChangeDisableBinding,
			},
			Bindings: []materialization.Binding[testResource]{
				{
					Resource: testResource{Table: "orders", Priority: 3},
					Source:   materialization.BindingSource{Name: "acmeCo/orders", NotBefore: "2026-01-01T00:00:00Z"},
					BindingOptions: materialization.BindingOptions{
						Priority:                   5,
						OnIncompatibleSchemaChange: materialization.OnIncompatibleSchemaChangeAbort,
						Fields:                     &materialization.BindingFields{Recommended: materialization.FieldDepthUnlimited, Exclude: []string{"_meta"}, Require: map[string]struct{}{"flow_document": {}, "_meta/op": {}}, GroupBy: []string{"id"}},
					},
				},
				{Resource: testResource{Table: "customers"}, Source: materialization.BindingSource{Name: "acmeCo/customers"}, BindingOptions: materialization.BindingOptions{Disable: true, Fields: &materialization.BindingFields{Recommended: "0"}}},
			},
		},
		"acmeCo/materialize-null-capture": {
			Endpoint: endpoint.ConnectorSpec[testConfig]{
				Connector: endpoint.Connector[testConfig]{
					Image:  testConnector.Image,
					Config: testConfig{Host: "warehouse.example.com"},
				},
			},
			SpecOptions: materialization.SpecOptions{
				Source:       &materialization.Source{},
				TargetNaming: &materialization.TargetNaming{Strategy: materialization.TargetNamingMatchSourceStructure},
			},
			Bindings: []materialization.Binding[testResource]{
				{Resource: testResource{Table: "orders"}, Source: materialization.BindingSource{Name: "acmeCo/orders"}},
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

				if err != nil || !bindingName.Equal(tftypes.NewValue(tftypes.String, binding.Source.Name)) {
					test.Errorf("bindings[%d].name = %v (%v), want the source %s", index, bindingName, err, binding.Source.Name)
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
			{Resource: testTopic{TopicName: "orders"}, Source: materialization.BindingSource{Name: "acmeCo/orders"}},
		},
	}

	codec, err := newCodec[testDekafDocument](connector, materializationType)

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
	if _, err := newCodec[materializationDocument[testConfig, testResource]](Connector{TypeName: "materialization_test"}, materializationType); err == nil {
		test.Error("newCodec accepted a connector without a default image")
	}
}

func TestMaterializationReadRefreshesBindingsBySource(test *testing.T) {
	var ctx context.Context = context.Background()
	var configureResp resource.ConfigureResponse
	var backend *testBackend = &testBackend{
		live: map[string]any{
			"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com"}}},
			"bindings": []any{
				map[string]any{"resource": map[string]any{"table": "orders", "priority": 3}, "source": map[string]any{"name": "acmeCo/orders", "notBefore": "2026-01-01T00:00:00Z"}},
				map[string]any{"resource": map[string]any{"table": "users"}, "source": "acmeCo/users"},
			},
		},
	}
	var state MaterializationSpec[testConfig, testResource] = MaterializationSpec[testConfig, testResource]{
		Endpoint: endpoint.ConnectorSpec[testConfig]{Connector: endpoint.Connector[testConfig]{Image: testNamedConnector.Image, Config: testConfig{Host: "db.example.com"}}},
		Bindings: []materialization.Binding[testResource]{{Resource: testResource{Table: "orders", Priority: 1}, Source: materialization.BindingSource{Name: "acmeCo/orders"}}},
	}
	var refreshed MaterializationSpec[testConfig, testResource] = state

	entry := NewMaterialization[testConfig, testResource](testNamedConnector)().(*entryResource[materializationDocument[testConfig, testResource], MaterializationSpec[testConfig, testResource]])

	entry.Configure(ctx, resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/"}}, &configureResp)

	refreshed.Bindings = []materialization.Binding[testResource]{
		{Resource: testResource{Table: "orders", Priority: 3}, Source: materialization.BindingSource{Name: "acmeCo/orders", NotBefore: "2026-01-01T00:00:00Z"}},
		{Resource: testResource{Table: "users"}, Source: materialization.BindingSource{Name: "acmeCo/users"}},
	}

	stateValue, err := entry.codec.Value("warehouse", state)

	if err != nil {
		test.Fatal(err)
	}

	want, err := entry.codec.Value("warehouse", refreshed)

	if err != nil {
		test.Fatal(err)
	}

	var readResp resource.ReadResponse = resource.ReadResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}}

	entry.Read(ctx, resource.ReadRequest{State: readResp.State}, &readResp)

	if readResp.Diagnostics.HasError() || readResp.Diagnostics.WarningsCount() > 0 {
		test.Fatalf("read: %v", readResp.Diagnostics)
	}

	if !readResp.State.Raw.Equal(want) {
		test.Errorf("refreshed state = %v, want %v", readResp.State.Raw, want)
	}
}

func TestResourceUpdateMaterializationBackfillTrigger(test *testing.T) {
	var ctx context.Context = context.Background()
	var configureResp resource.ConfigureResponse
	var backend *testBackend = &testBackend{
		live: map[string]any{
			"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com"}}},
			"bindings": []any{
				map[string]any{"resource": map[string]any{"table": "events"}, "source": "acmeCo/events", "backfill": 1, "disable": true},
				map[string]any{"resource": map[string]any{"table": "orders"}, "source": "acmeCo/orders", "backfill": 4},
			},
		},
	}
	var spec MaterializationSpec[testConfig, testResource] = MaterializationSpec[testConfig, testResource]{
		Endpoint: endpoint.ConnectorSpec[testConfig]{Connector: endpoint.Connector[testConfig]{Image: testNamedConnector.Image, Config: testConfig{Host: "db.example.com"}}},
		Bindings: []materialization.Binding[testResource]{
			{Resource: testResource{Table: "events"}, Source: materialization.BindingSource{Name: "acmeCo/events"}, Disable: true},
			{Resource: testResource{Table: "orders"}, Source: materialization.BindingSource{Name: "acmeCo/orders"}},
		},
	}

	entry := NewMaterialization[testConfig, testResource](testNamedConnector)().(*entryResource[materializationDocument[testConfig, testResource], MaterializationSpec[testConfig, testResource]])

	entry.Configure(ctx, resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/"}}, &configureResp)

	value, err := entry.codec.Value("warehouse", spec)

	if err != nil {
		test.Fatal(err)
	}

	priorValue, err := withValue(value, testEntryBackfillPath, map[string]tftypes.Value{"trigger": tftypes.NewValue(tftypes.Number, 2)})

	if err != nil {
		test.Fatal(err)
	}

	planValue, err := withValue(value, testEntryBackfillPath, map[string]tftypes.Value{"trigger": tftypes.NewValue(tftypes.Number, 3)})

	if err != nil {
		test.Fatal(err)
	}

	loweredValue, err := withValue(value, testEntryBackfillPath, map[string]tftypes.Value{"trigger": tftypes.NewValue(tftypes.Number, 1)})

	if err != nil {
		test.Fatal(err)
	}

	var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

	entry.Update(ctx, resource.UpdateRequest{
		State: tfsdk.State{Schema: entry.codec.Schema(), Raw: priorValue},
		Plan:  tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
	}, &updateResp)

	if updateResp.Diagnostics.HasError() {
		test.Fatalf("update: %v", updateResp.Diagnostics)
	}

	var published map[string]any = map[string]any{}

	bindings, _ := backend.published["bindings"].([]any)

	for _, binding := range bindings {
		object, _ := binding.(map[string]any)

		published[catalog.BindingCollection(binding)] = object["backfill"]
	}

	if !reflect.DeepEqual(published, map[string]any{"acmeCo/events": 1, "acmeCo/orders": 5}) || backend.resets != nil {
		test.Errorf("published backfills %v and reset %v, want the disabled events kept, orders raised by one and no reset", published, backend.resets)
	}

	if _, has := backend.published["backfill"]; has {
		test.Errorf("published %v, want the backfill block left out of the spec", backend.published)
	}

	warnings := entry.warnEntryBackfill(planValue, priorValue, entry.newPublicationReader(planValue, priorValue, entry.newLiveReader(ctx, priorValue)))

	if len(warnings) != 1 || !strings.HasPrefix(warnings[0].Summary(), "Backfill of every binding in acmeCo/warehouse") || !strings.Contains(warnings[0].Detail(), "- acmeCo/orders\n\n") {
		test.Errorf("warnings = %v, want a backfill warning listing orders only", warnings)
	}

	if warnings := entry.warnEntryBackfill(priorValue, priorValue, entry.newPublicationReader(priorValue, priorValue, entry.newLiveReader(ctx, priorValue))); len(warnings) != 0 {
		test.Errorf("warnings = %v, want none without a raise", warnings)
	}

	diagnostics := validateBackfillTriggers(loweredValue, priorValue, entry.catalogType)

	if len(diagnostics) != 1 || diagnostics[0].Summary() != "Backfill trigger lowered" || !strings.Contains(diagnostics[0].Detail(), "the materialization") {
		test.Errorf("diagnostics = %v, want the lowered trigger of the materialization rejected", diagnostics)
	}
}

func TestMaterializationWarnsAboutSourceCaptureBindings(test *testing.T) {
	var configureResp resource.ConfigureResponse
	var sourceCapture string = "acmeCo/source-postgres"
	var spec MaterializationSpec[testConfig, testResource] = MaterializationSpec[testConfig, testResource]{
		Endpoint: endpoint.ConnectorSpec[testConfig]{Connector: endpoint.Connector[testConfig]{Image: testNamedConnector.Image, Config: testConfig{Host: "db.example.com"}}},
		Bindings: []materialization.Binding[testResource]{
			{Resource: testResource{Table: "orders"}, Source: materialization.BindingSource{Name: "acmeCo/orders"}},
			{Resource: testResource{Table: "users"}, Source: materialization.BindingSource{Name: "acmeCo/users"}},
		},
	}

	entry := NewMaterialization[testConfig, testResource](testNamedConnector)().(*entryResource[materializationDocument[testConfig, testResource], MaterializationSpec[testConfig, testResource]])

	entry.Configure(context.Background(), resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: &testBackend{}, Tenant: "acmeCo/"}}, &configureResp)

	values := func(source *materialization.Source) (tftypes.Value, tftypes.Value) {
		var full MaterializationSpec[testConfig, testResource] = spec
		var removed MaterializationSpec[testConfig, testResource] = spec

		full.Source, removed.Source = source, source
		removed.Bindings = spec.Bindings[:1]

		fullValue, err := entry.codec.Value("warehouse", full)

		if err != nil {
			test.Fatal(err)
		}

		removedValue, err := entry.codec.Value("warehouse", removed)

		if err != nil {
			test.Fatal(err)
		}

		return fullValue, removedValue
	}

	unlinked, unlinkedRemoved := values(nil)
	linked, linkedRemoved := values(&materialization.Source{Capture: &sourceCapture})

	var created tftypes.Value = tftypes.NewValue(unlinked.Type(), nil)

	for name, state := range map[string]tftypes.Value{"creation": created, "linking update": unlinked} {
		warnings := entry.warnAddedBindings(linked, state)

		if len(warnings) != 1 || !warnings[0].(diag.DiagnosticWithPath).Path().Equal(path.Root("source").AtName("capture")) ||
			!strings.HasPrefix(warnings[0].Summary(), "Source capture adds unmanaged bindings in acmeCo/warehouse") ||
			!strings.Contains(warnings[0].Detail(), "not per-binding overrides such as fields.require") {
			test.Errorf("%s: warnings = %v, want a source.capture warning listing the settings added bindings lack", name, warnings)
		}
	}

	if warnings := entry.warnAddedBindings(linked, linked); len(warnings) != 0 {
		test.Errorf("warnings = %v, want none while the capture stays linked", warnings)
	}

	if warnings := entry.warnAddedBindings(unlinked, created); len(warnings) != 0 {
		test.Errorf("warnings = %v, want none without a source capture", warnings)
	}

	warnings := entry.warnRemovedBindings(linkedRemoved, linked)

	if len(warnings) != 1 || !strings.Contains(warnings[0].Detail(), "- acmeCo/users\n") || !strings.Contains(warnings[0].Detail(), "To stop materializing one") {
		test.Errorf("warnings = %v, want a removed binding warning listing users", warnings)
	}

	if warnings := entry.warnRemovedBindings(unlinkedRemoved, unlinked); len(warnings) != 0 {
		test.Errorf("warnings = %v, want none without a source capture", warnings)
	}
}

func TestMaterializationReadShowsLostFieldDepth(test *testing.T) {
	var ctx context.Context = context.Background()
	var linked string = "acmeCo/source-postgres"

	expectations := map[string]struct {
		state materialization.FieldDepth
		want  materialization.FieldDepth
	}{
		"drops an unlimited depth Estuary lost": {materialization.FieldDepthUnlimited, ""},
		"drops another depth Estuary lost":      {"1", ""},
	}

	for name, expected := range expectations {
		var configureResp resource.ConfigureResponse
		var backend *testBackend = &testBackend{
			live: map[string]any{
				"source":   map[string]any{"capture": linked, "deltaUpdates": false},
				"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com"}}},
				"bindings": []any{},
			},
		}
		var spec MaterializationSpec[testConfig, testResource] = MaterializationSpec[testConfig, testResource]{
			Endpoint: endpoint.ConnectorSpec[testConfig]{Connector: endpoint.Connector[testConfig]{Image: testNamedConnector.Image, Config: testConfig{Host: "db.example.com"}}},
		}

		entry := NewMaterialization[testConfig, testResource](testNamedConnector)().(*entryResource[materializationDocument[testConfig, testResource], MaterializationSpec[testConfig, testResource]])

		entry.Configure(ctx, resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/"}}, &configureResp)

		spec.Source = &materialization.Source{Capture: &linked, FieldsRecommended: expected.state}

		stateValue, err := entry.codec.Value("warehouse", spec)

		if err != nil {
			test.Fatal(err)
		}

		spec.Source.FieldsRecommended = expected.want

		want, err := entry.codec.Value("warehouse", spec)

		if err != nil {
			test.Fatal(err)
		}

		var readResp resource.ReadResponse = resource.ReadResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}}

		entry.Read(ctx, resource.ReadRequest{State: readResp.State}, &readResp)

		if readResp.Diagnostics.HasError() || !readResp.State.Raw.Equal(want) {
			test.Errorf("%s: refreshed state = %v (%v), want %v", name, readResp.State.Raw, readResp.Diagnostics, want)
		}
	}
}
