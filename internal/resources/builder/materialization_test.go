package builder

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	materializepostgres "github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/postgres"
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

	codec, err := newCodec[materializationDocument[testConfig, testResource]](testConnector, materializationType[testConfig, testResource]())

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

	codec, err := newCodec[materializationDocument[testConfig, testResource]](testConnector, materializationType[testConfig, testResource]())

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

	codec, err := newCodec[testDekafDocument](connector, materializationType[testConfig, testTopic]())

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

	NewMaterializationFromDocument[testDekafDocument, testDekafSpec, testConfig, testTopic](connector)().Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "estuary"}, &metadataResp)

	if metadataResp.TypeName != "estuary_materialization_test_dekaf" {
		test.Errorf("type name = %q, want estuary_materialization_test_dekaf", metadataResp.TypeName)
	}
}

func TestNewCodecRequiresConnectorImage(test *testing.T) {
	if _, err := newCodec[materializationDocument[testConfig, testResource]](Connector{TypeName: "materialization_test"}, materializationType[testConfig, testResource]()); err == nil {
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

	if err == nil {
		stateValue, err = withValue(stateValue, keepUnmanagedBindingsPath, false)
	}

	if err != nil {
		test.Fatal(err)
	}

	want, err := entry.codec.Value("warehouse", refreshed)

	if err == nil {
		want, err = withValue(want, keepUnmanagedBindingsPath, false)
	}

	if err != nil {
		test.Fatal(err)
	}

	var readResp resource.ReadResponse = resource.ReadResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}}

	entry.Read(ctx, resource.ReadRequest{State: readResp.State}, &readResp)

	if readResp.Diagnostics.HasError() || readResp.Diagnostics.WarningsCount() > 0 {
		test.Fatalf("read: %v", readResp.Diagnostics)
	}

	refreshedState, err := nullAttributes(readResp.State.Raw, computedByRead)

	if err != nil {
		test.Fatal(err)
	}

	if !refreshedState.Equal(want) {
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
			!strings.Contains(warnings[0].Detail(), "such as fields.require") {
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
		"keeps the unlimited default Estuary omits": {materialization.FieldDepthUnlimited, materialization.FieldDepthUnlimited},
		"drops another depth Estuary lost":          {"1", ""},
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

		refreshed, err := nullAttributes(readResp.State.Raw, computedByRead)

		if err != nil {
			test.Fatal(err)
		}

		if readResp.Diagnostics.HasError() || !refreshed.Equal(want) {
			test.Errorf("%s: refreshed state = %v (%v), want %v", name, readResp.State.Raw, readResp.Diagnostics, want)
		}
	}
}

func TestPlanBindingFieldsOmitted(test *testing.T) {
	var unlimited *materialization.BindingFields = &materialization.BindingFields{Recommended: materialization.FieldDepthUnlimited}
	var depthOne *materialization.BindingFields = &materialization.BindingFields{Recommended: "1"}

	var spec MaterializationSpec[testConfig, testResource] = MaterializationSpec[testConfig, testResource]{
		Endpoint: endpoint.ConnectorSpec[testConfig]{Connector: endpoint.Connector[testConfig]{Image: testNamedConnector.Image, Config: testConfig{Host: "db.example.com"}}},
	}

	entry := NewMaterialization[testConfig, testResource](testNamedConnector)().(*entryResource[materializationDocument[testConfig, testResource], MaterializationSpec[testConfig, testResource]])

	binding := func(collection string, fields *materialization.BindingFields) materialization.Binding[testResource] {
		return materialization.Binding[testResource]{Resource: testResource{Table: collection}, Source: materialization.BindingSource{Name: "acmeCo/" + collection}, Fields: fields}
	}

	value := func(bindings ...materialization.Binding[testResource]) tftypes.Value {
		var current MaterializationSpec[testConfig, testResource] = spec

		current.Bindings = bindings

		converted, err := entry.codec.Value("warehouse", current)

		if err != nil {
			test.Fatal(err)
		}

		return converted
	}

	var state tftypes.Value = value(binding("orders", unlimited), binding("users", unlimited))
	var live tftypes.Value = value(binding("orders", unlimited), binding("users", unlimited), binding("events", depthOne))
	var config tftypes.Value = value(binding("orders", nil), binding("users", depthOne), binding("events", nil), binding("sessions", nil))

	planned, err := planBindingFields(config, config, state, func() (tftypes.Value, bool) { return live, true })

	if err != nil {
		test.Fatal(err)
	}

	if want := value(binding("orders", unlimited), binding("users", depthOne), binding("events", depthOne), binding("sessions", unlimited)); !planned.Equal(want) {
		test.Errorf("planned = %v, want the stored selection for orders, the configured one for users, the live one for the taken over events and Estuary's default for sessions", planned)
	}

	unread, err := planBindingFields(config, config, state, func() (tftypes.Value, bool) { return tftypes.Value{}, false })

	if err != nil {
		test.Fatal(err)
	}

	if want := value(binding("orders", unlimited), binding("users", depthOne), binding("events", nil), binding("sessions", nil)); !unread.Equal(want) {
		test.Errorf("planned with an unreadable live spec = %v, want no selection for the bindings state doesn't hold", unread)
	}

	created, err := planBindingFields(config, config, tftypes.NewValue(config.Type(), nil), func() (tftypes.Value, bool) { return tftypes.Value{}, false })

	if err != nil {
		test.Fatal(err)
	}

	if want := value(binding("orders", unlimited), binding("users", depthOne), binding("events", unlimited), binding("sessions", unlimited)); !created.Equal(want) {
		test.Errorf("planned for a new entry = %v, want Estuary's default for every binding configured without fields", created)
	}

	var sessionsSource *tftypes.AttributePath = bindingsPath.WithElementKeyInt(3).WithAttributeName(bindingSourceAttribute)

	unknownConfig, err := withValue(config, sessionsSource, tftypes.UnknownValue)

	if err != nil {
		test.Fatal(err)
	}

	unknownCollection, err := planBindingFields(unknownConfig, unknownConfig, tftypes.NewValue(config.Type(), nil), func() (tftypes.Value, bool) { return tftypes.Value{}, false })

	if err != nil {
		test.Fatal(err)
	}

	wantUnknown, err := withValue(value(binding("orders", unlimited), binding("users", depthOne), binding("events", unlimited), binding("sessions", unlimited)), sessionsSource, tftypes.UnknownValue)

	if err != nil {
		test.Fatal(err)
	}

	if !unknownCollection.Equal(wantUnknown) {
		test.Errorf("planned for a new binding of a collection unknown until apply = %v, want Estuary's default", unknownCollection)
	}
}

func TestDefaultBindingFields(test *testing.T) {
	var fieldsType tftypes.Object = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		bindingFieldsRecommendedAttribute: tftypes.String,
		"require":                         tftypes.Set{ElementType: tftypes.String},
		"exclude":                         tftypes.List{ElementType: tftypes.String},
	}}

	fields, err := defaultBindingFields(fieldsType)

	if err != nil {
		test.Fatal(err)
	}

	var want tftypes.Value = tftypes.NewValue(fieldsType, map[string]tftypes.Value{
		bindingFieldsRecommendedAttribute: tftypes.NewValue(tftypes.String, "unlimited"),
		"require":                         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"exclude":                         tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
	})

	if !fields.Equal(want) {
		test.Errorf("defaultBindingFields = %v, want only recommended = \"unlimited\" set", fields)
	}

	for label, invalid := range map[string]tftypes.Type{
		"not an object":       tftypes.String,
		"without recommended": tftypes.Object{AttributeTypes: map[string]tftypes.Type{"require": tftypes.Set{ElementType: tftypes.String}}},
	} {
		if _, err := defaultBindingFields(invalid); err == nil {
			test.Errorf("defaultBindingFields of fields %s returned no error", label)
		}
	}
}

func TestMaterializationTakeoverWarningKeepsEstuaryFields(test *testing.T) {
	var ctx context.Context = context.Background()
	var configureResp resource.ConfigureResponse
	var linked string = "acmeCo/source-postgres"
	var unlimited *materialization.BindingFields = &materialization.BindingFields{Recommended: materialization.FieldDepthUnlimited}

	var spec MaterializationSpec[testConfig, testResource] = MaterializationSpec[testConfig, testResource]{
		Endpoint:    endpoint.ConnectorSpec[testConfig]{Connector: endpoint.Connector[testConfig]{Image: testNamedConnector.Image, Config: testConfig{Host: "db.example.com"}}},
		SpecOptions: materialization.SpecOptions{Source: &materialization.Source{Capture: &linked}},
	}

	var users materialization.Binding[testResource] = materialization.Binding[testResource]{
		Resource: testResource{Table: "users"}, Source: materialization.BindingSource{Name: "acmeCo/users"}, Fields: unlimited,
	}

	entry := NewMaterialization[testConfig, testResource](testNamedConnector)().(*entryResource[materializationDocument[testConfig, testResource], MaterializationSpec[testConfig, testResource]])

	entry.Configure(ctx, resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: &testBackend{}, Tenant: "acmeCo/"}}, &configureResp)

	state, err := entry.codec.Value("warehouse", spec)

	if err != nil {
		test.Fatal(err)
	}

	spec.Bindings = []materialization.Binding[testResource]{users}

	live, err := entry.codec.Value("warehouse", spec)

	if err != nil {
		test.Fatal(err)
	}

	for name, expected := range map[string]struct {
		trigger any
		want    string
	}{
		"lists the raised trigger":          {1, "- acmeCo/users (bindings[0]):\n    bindings[0].backfill: (unset) -> (changed)\n"},
		"lists no change without a trigger": {nil, "- acmeCo/users (bindings[0]): no field changes\n"},
	} {
		plan, err := withValue(live, bindingsPath.WithElementKeyInt(0).WithAttributeName(backfillAttribute),
			map[string]tftypes.Value{backfillTriggerAttribute: tftypes.NewValue(tftypes.Number, expected.trigger)})

		if err != nil {
			test.Fatal(err)
		}

		if expected.trigger == nil {
			plan = live
		}

		var readLive liveReader = func() (tftypes.Value, bool) { return live, true }

		warnings := entry.warnTakenOverBindings(plan, state, readLive)

		if len(warnings) != 1 || !strings.Contains(warnings[0].Detail(), expected.want) || strings.Contains(warnings[0].Detail(), "bindings[0].fields") {
			test.Errorf("%s: warnings = %v, want %q without fields", name, warnings, expected.want)
		}
	}
}

type testSchemaResource struct {
	Table  string `yaml:"table"`
	Schema string `yaml:"schema,omitempty" x-schema-name:"true"`
}

func TestPlanBindingSchemasOmitted(test *testing.T) {
	var spec MaterializationSpec[testWarehouseConfig, testSchemaResource] = MaterializationSpec[testWarehouseConfig, testSchemaResource]{
		Endpoint: endpoint.ConnectorSpec[testWarehouseConfig]{Connector: endpoint.Connector[testWarehouseConfig]{Image: testNamedConnector.Image, Config: testWarehouseConfig{Host: "db.example.com"}}},
		SpecOptions: materialization.SpecOptions{
			TargetNaming: &materialization.TargetNaming{Strategy: materialization.TargetNamingMatchSourceStructure, SchemaTemplate: "raw_{{schema}}"},
		},
	}

	entry := NewMaterialization[testWarehouseConfig, testSchemaResource](testNamedConnector)().(*entryResource[materializationDocument[testWarehouseConfig, testSchemaResource], MaterializationSpec[testWarehouseConfig, testSchemaResource]])

	binding := func(collection string, schema string) materialization.Binding[testSchemaResource] {
		return materialization.Binding[testSchemaResource]{
			Resource: testSchemaResource{Table: collection, Schema: schema},
			Source:   materialization.BindingSource{Name: "acmeCo/sales/" + collection},
			Fields:   &materialization.BindingFields{Recommended: materialization.FieldDepthUnlimited},
		}
	}

	value := func(bindings ...materialization.Binding[testSchemaResource]) tftypes.Value {
		var current MaterializationSpec[testWarehouseConfig, testSchemaResource] = spec

		current.Bindings = bindings

		converted, err := entry.codec.Value("warehouse", current)

		if err != nil {
			test.Fatal(err)
		}

		return converted
	}

	var state tftypes.Value = value(binding("orders", "legacy"), binding("users", ""))
	var live tftypes.Value = value(binding("orders", "legacy"), binding("users", ""), binding("events", "dashboard"))
	var config tftypes.Value = value(binding("orders", ""), binding("users", ""), binding("events", ""), binding("sessions", ""), binding("carts", "custom"))

	planned, err := entry.catalogType.planBindings(config, config, state, func() (tftypes.Value, bool) { return live, true })

	if err != nil {
		test.Fatal(err)
	}

	if want := value(binding("orders", "legacy"), binding("users", ""), binding("events", "dashboard"), binding("sessions", "raw_sales"), binding("carts", "custom")); !planned.Equal(want) {
		test.Errorf("planned = %v, want the stored schema for orders and users, the live one for the taken over events, target_naming for sessions and the configured one for carts", planned)
	}

	unread, err := entry.catalogType.planBindings(config, config, state, func() (tftypes.Value, bool) { return tftypes.Value{}, false })

	if err != nil {
		test.Fatal(err)
	}

	for _, index := range []int{2, 3} {
		var unreadSchema *tftypes.AttributePath = bindingsPath.WithElementKeyInt(index).WithAttributeName(bindingResourceAttribute).WithAttributeName("schema")

		if schema, err := attributeValue(unread, unreadSchema); err != nil || schema.IsKnown() {
			test.Errorf("planned schema of binding %d with an unreadable live spec = %v (%v), want unknown, not a target_naming one replacing the live schema", index, schema, err)
		}
	}

	var sessionsSource *tftypes.AttributePath = bindingsPath.WithElementKeyInt(3).WithAttributeName(bindingSourceAttribute)
	var sessionsSchema *tftypes.AttributePath = bindingsPath.WithElementKeyInt(3).WithAttributeName(bindingResourceAttribute).WithAttributeName("schema")

	unknownConfig, err := withValue(config, sessionsSource, tftypes.UnknownValue)

	if err != nil {
		test.Fatal(err)
	}

	unknownCollection, err := entry.catalogType.planBindings(unknownConfig, unknownConfig, state, func() (tftypes.Value, bool) { return live, true })

	if err != nil {
		test.Fatal(err)
	}

	if schema, err := attributeValue(unknownCollection, sessionsSchema); err != nil || schema.IsKnown() {
		test.Errorf("planned schema of a binding with an unknown collection = %v (%v), want unknown", schema, err)
	}

	resource, _ := entry.codec.Schema().Attributes[bindingsAttribute].(schema.ListNestedAttribute).NestedObject.Attributes[bindingResourceAttribute].(schema.SingleNestedAttribute)

	if schemaName, _ := resource.Attributes["schema"].(schema.StringAttribute); !schemaName.Optional || !schemaName.Computed {
		test.Errorf("resource schema attribute = %+v, want optional and computed", schemaName)
	}
}

func TestWarnNamingChange(test *testing.T) {
	var previous *materialization.TargetNaming = &materialization.TargetNaming{Strategy: materialization.TargetNamingMatchSourceStructure, SchemaTemplate: "raw_{{schema}}"}
	var current *materialization.TargetNaming = &materialization.TargetNaming{Strategy: materialization.TargetNamingMatchSourceStructure}

	entry := NewMaterialization[testWarehouseConfig, testSchemaResource](testNamedConnector)().(*entryResource[materializationDocument[testWarehouseConfig, testSchemaResource], MaterializationSpec[testWarehouseConfig, testSchemaResource]])

	value := func(naming *materialization.TargetNaming) tftypes.Value {
		converted, err := entry.codec.Value("warehouse", MaterializationSpec[testWarehouseConfig, testSchemaResource]{
			Endpoint:    endpoint.ConnectorSpec[testWarehouseConfig]{Connector: endpoint.Connector[testWarehouseConfig]{Image: testNamedConnector.Image, Config: testWarehouseConfig{Host: "db.example.com"}}},
			SpecOptions: materialization.SpecOptions{TargetNaming: naming},
		})

		if err != nil {
			test.Fatal(err)
		}

		return converted
	}

	if diagnostics := entry.warnNamingChange(value(current), value(previous)); len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Detail(), "only names the bindings added from now on") {
		test.Errorf("diagnostics of a target_naming change = %v, want one warning", diagnostics)
	}

	if diagnostics := entry.warnNamingChange(value(previous), value(previous)); len(diagnostics) != 0 {
		test.Errorf("diagnostics of an unchanged target_naming = %v, want none", diagnostics)
	}

	if diagnostics := entry.warnNamingChange(value(current), tftypes.NewValue(value(current).Type(), nil)); len(diagnostics) != 0 {
		test.Errorf("diagnostics of a new materialization = %v, want none", diagnostics)
	}
}

type testWarehouseConfig struct {
	Host          string `yaml:"host"`
	DefaultSchema string `yaml:"default_schema" x-default-namespace:"true"`
}

type testWarehouseResource struct {
	Table  string `yaml:"table" x-collection-name:"true"`
	Schema string `yaml:"schema,omitempty" x-schema-name:"true"`
}

func TestWarnMovedTables(test *testing.T) {
	entry := NewMaterialization[testWarehouseConfig, testWarehouseResource](testNamedConnector)().(*entryResource[materializationDocument[testWarehouseConfig, testWarehouseResource], MaterializationSpec[testWarehouseConfig, testWarehouseResource]])

	binding := func(collection string, schema string, table string) materialization.Binding[testWarehouseResource] {
		return materialization.Binding[testWarehouseResource]{
			Resource: testWarehouseResource{Table: table, Schema: schema},
			Source:   materialization.BindingSource{Name: "acmeCo/sales/" + collection},
			Fields:   &materialization.BindingFields{Recommended: materialization.FieldDepthUnlimited},
		}
	}

	value := func(bindings ...materialization.Binding[testWarehouseResource]) tftypes.Value {
		converted, err := entry.codec.Value("warehouse", MaterializationSpec[testWarehouseConfig, testWarehouseResource]{
			Endpoint: endpoint.ConnectorSpec[testWarehouseConfig]{Connector: endpoint.Connector[testWarehouseConfig]{Image: testNamedConnector.Image, Config: testWarehouseConfig{Host: "db.example.com", DefaultSchema: "public"}}},
			Bindings: bindings,
		})

		if err != nil {
			test.Fatal(err)
		}

		return converted
	}

	var live tftypes.Value = value(binding("orders", "", "orders"), binding("users", "raw", "users"), binding("events", "", "events"), binding("refunds", "raw", "refunds"), binding("carts", "raw", "carts"))
	var plan tftypes.Value = value(binding("orders", "", "orders_v2"), binding("users", "sales", "users"), binding("events", "public", "events"), binding("refunds", "raw", "refunds"), binding("carts", "RAW", "CARTS"), binding("sessions", "", "sessions"))
	var readLive liveReader = func() (tftypes.Value, bool) { return live, true }

	want := []string{
		"acmeCo/sales/orders: public.orders -> public.orders_v2",
		"acmeCo/sales/users: raw.users -> sales.users",
	}

	if moved := entry.catalogType.movedTables(plan, readLive); !reflect.DeepEqual(moved, want) {
		test.Errorf("moved = %q, want orders and users, without events, whose schema is the endpoint one, the unchanged refunds, carts, which only changes case, nor the new sessions", moved)
	}

	if diagnostics := entry.warnMovedTables(plan, readLive); len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Detail(), "- acmeCo/sales/users: raw.users -> sales.users") {
		test.Errorf("diagnostics = %v, want one warning listing the moved bindings", diagnostics)
	}

	if diagnostics := entry.warnMovedTables(live, readLive); len(diagnostics) != 0 {
		test.Errorf("diagnostics of an unchanged plan = %v, want none", diagnostics)
	}

	if diagnostics := entry.warnMovedTables(plan, func() (tftypes.Value, bool) { return tftypes.Value{}, false }); len(diagnostics) != 0 {
		test.Errorf("diagnostics with an unreadable live spec = %v, want none", diagnostics)
	}
}

func TestSchemaNameRequiresDefaultNamespace(test *testing.T) {
	if _, err := newCodec[materializationDocument[testConfig, testSchemaResource]](testNamedConnector, materializationType[testConfig, testSchemaResource]()); err == nil || !strings.Contains(err.Error(), "x-default-namespace") {
		test.Errorf("codec of a schema resource without a default namespace config = %v, want an x-default-namespace error", err)
	}

	if _, err := newCodec[materializationDocument[testWarehouseConfig, testSchemaResource]](testNamedConnector, materializationType[testWarehouseConfig, testSchemaResource]()); err != nil {
		test.Errorf("codec of a schema resource with a default namespace config: %v", err)
	}
}

func TestResourceUpdateRequiresBackfillsOfStandardSwitches(test *testing.T) {
	var ctx context.Context = context.Background()

	expectations := map[string]struct {
		prior    bool
		live     bool
		planned  bool
		disabled bool
		trigger  any
		entry    any
		rejected bool
		sales    any
	}{
		"rejects a switch to standard updates without a raise":   {true, true, false, false, 1, nil, true, nil},
		"rejects a switch from delta updates set outside":        {false, true, false, false, 1, nil, true, nil},
		"backfills a switch to standard updates with a raise":    {true, true, false, false, 2, nil, false, 4},
		"backfills a switch with the materialization trigger":    {true, true, false, false, 1, 1, false, 4},
		"accepts a switch to delta updates without a raise":      {false, false, true, false, 1, nil, false, 3},
		"accepts a switch of a disabled binding without a raise": {true, true, false, true, 1, nil, false, 3},
	}

	for name, expected := range expectations {
		var configureResp resource.ConfigureResponse
		var backend *testBackend = &testBackend{live: map[string]any{
			"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"address": "db:5432", "user": "flow"}}},
			"bindings": []any{
				map[string]any{"resource": map[string]any{"table": "sales", "delta_updates": expected.live}, "source": "acmeCo/sales", "backfill": 3, "disable": expected.disabled},
			},
		}}

		entry := NewMaterialization[materializepostgres.Config, materializepostgres.Resource](testNamedConnector)().(*entryResource[materializationDocument[materializepostgres.Config, materializepostgres.Resource], MaterializationSpec[materializepostgres.Config, materializepostgres.Resource]])

		entry.Configure(ctx, resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/"}}, &configureResp)

		var prior MaterializationSpec[materializepostgres.Config, materializepostgres.Resource] = MaterializationSpec[materializepostgres.Config, materializepostgres.Resource]{
			Endpoint: endpoint.ConnectorSpec[materializepostgres.Config]{Connector: endpoint.Connector[materializepostgres.Config]{
				Image:  testNamedConnector.Image,
				Config: materializepostgres.Config{Address: "db:5432", User: "flow"},
			}},
			Bindings: []materialization.Binding[materializepostgres.Resource]{
				{Resource: materializepostgres.Resource{Table: "sales", DeltaUpdates: expected.prior}, Source: materialization.BindingSource{Name: "acmeCo/sales"}, Disable: expected.disabled},
			},
		}
		var planned MaterializationSpec[materializepostgres.Config, materializepostgres.Resource] = prior

		planned.Bindings = []materialization.Binding[materializepostgres.Resource]{
			{Resource: materializepostgres.Resource{Table: "sales", DeltaUpdates: expected.planned}, Source: materialization.BindingSource{Name: "acmeCo/sales"}, Disable: expected.disabled},
		}

		priorValue, err := entry.codec.Value("warehouse", prior)

		if err == nil {
			priorValue, err = withValue(priorValue, testBackfillBlockPath, map[string]tftypes.Value{"trigger": tftypes.NewValue(tftypes.Number, 1)})
		}

		if err == nil && expected.entry != nil {
			priorValue, err = withValue(priorValue, testEntryBackfillPath, map[string]tftypes.Value{"trigger": tftypes.NewValue(tftypes.Number, nil)})
		}

		if err != nil {
			test.Fatal(err)
		}

		planValue, err := entry.codec.Value("warehouse", planned)

		if err == nil {
			planValue, err = withValue(planValue, testBackfillBlockPath, map[string]tftypes.Value{"trigger": tftypes.NewValue(tftypes.Number, expected.trigger)})
		}

		if err == nil && expected.entry != nil {
			planValue, err = withValue(planValue, testEntryBackfillPath, map[string]tftypes.Value{"trigger": tftypes.NewValue(tftypes.Number, expected.entry)})
		}

		if err != nil {
			test.Fatal(err)
		}

		diagnostics := entry.validateMissingBackfills(planValue, entry.newPublicationReader(planValue, priorValue, entry.newLiveReader(ctx, priorValue)))

		if diagnostics.HasError() != expected.rejected || (expected.rejected && !strings.Contains(diagnostics[0].Detail(), "resource.delta_updates of acmeCo/sales is true in Estuary")) {
			test.Errorf("%s: plan diagnostics = %v, want rejected %v", name, diagnostics, expected.rejected)
		}

		if len(diagnostics) == 1 {
			var attributePath path.Path = diagnostics[0].(diag.DiagnosticWithPath).Path()

			if !attributePath.Equal(path.Root("bindings").AtListIndex(0).AtName("resource").AtName("delta_updates")) {
				test.Errorf("%s: diagnostic path = %s, want bindings[0].resource.delta_updates", name, attributePath)
			}
		}

		var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

		entry.Update(ctx, resource.UpdateRequest{
			State: tfsdk.State{Schema: entry.codec.Schema(), Raw: priorValue},
			Plan:  tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
		}, &updateResp)

		if updateResp.Diagnostics.HasError() != expected.rejected {
			test.Errorf("%s: apply diagnostics = %v, want rejected %v", name, updateResp.Diagnostics, expected.rejected)
			continue
		}

		if expected.rejected {
			if backend.published != nil {
				test.Errorf("%s: published %v, want nothing published", name, backend.published)
			}

			continue
		}

		bindings, _ := backend.published["bindings"].([]any)

		if len(bindings) != 1 || bindings[0].(map[string]any)["backfill"] != expected.sales {
			test.Errorf("%s: published bindings = %v, want the sales backfill counter %v", name, bindings, expected.sales)
		}
	}
}
