package builder

import (
	"context"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCodecLiveBindingsSchema(test *testing.T) {
	var ctx context.Context = context.Background()

	entry := newTestCaptureEntry(&testBackend{})

	bindings, isBindings := entry.codec.Schema().Attributes[bindingsAttribute].(schema.ListNestedAttribute)
	live, isLive := entry.codec.Schema().Attributes[liveBindingsAttribute].(schema.ListAttribute)

	if !isBindings || !isLive {
		test.Fatalf("bindings = %T, live_bindings = %T, want a nested list and a list", entry.codec.Schema().Attributes[bindingsAttribute], entry.codec.Schema().Attributes[liveBindingsAttribute])
	}

	if !live.IsComputed() || live.IsRequired() || live.IsOptional() {
		test.Errorf("live_bindings = %+v, want read-only", live)
	}

	if !live.GetType().TerraformType(ctx).Equal(bindings.GetType().TerraformType(ctx)) {
		test.Errorf("live_bindings type = %v, want the bindings type %v", live.GetType().TerraformType(ctx), bindings.GetType().TerraformType(ctx))
	}

	codec, err := newCodec[materializationDocument[testConfig, testResource], MaterializationSpec[testConfig, testResource]](testNamedConnector, materializationType)

	if err != nil {
		test.Fatal(err)
	}

	if _, found := codec.Schema().Attributes[liveBindingsAttribute]; found {
		test.Error("materialization has live_bindings, want it only on captures")
	}
}

func TestResourceReadSetsLiveBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var discovering CaptureSpec[testConfig, testResource] = testCaptureSpec
	var orders capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}
	var events capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "events"}, Target: "acmeCo/events"}

	discovering.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true}
	discovering.Bindings = []capture.Binding[testResource]{orders}

	expectations := map[string]struct {
		live []any
		want []capture.Binding[testResource]
	}{
		"lists the unmanaged bindings too": {
			[]any{map[string]any{"resource": map[string]any{"table": "events"}, "target": "acmeCo/events"}, map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"}},
			[]capture.Binding[testResource]{events, orders},
		},
		"lists none of an entry without bindings": {
			nil,
			[]capture.Binding[testResource]{},
		},
	}

	for name, expected := range expectations {
		var backend *testBackend = &testBackend{live: liveCapture(expected.live...)}
		var listed CaptureSpec[testConfig, testResource] = discovering

		backend.live["autoDiscover"] = map[string]any{"addNewBindings": true}

		entry := newTestCaptureEntry(backend)

		listed.Bindings = expected.want

		stateValue, err := entry.codec.Value("movies", discovering)

		if err != nil {
			test.Fatal(err)
		}

		listedValue, err := entry.codec.Value("movies", listed)

		if err != nil {
			test.Fatal(err)
		}

		wantState, err := withLiveBindings(stateValue, listedValue)

		if err != nil {
			test.Fatal(err)
		}

		var readResp resource.ReadResponse = resource.ReadResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}}

		entry.Read(ctx, resource.ReadRequest{State: readResp.State}, &readResp)

		if readResp.Diagnostics.HasError() || readResp.Diagnostics.WarningsCount() > 0 {
			test.Errorf("%s: read: %v", name, readResp.Diagnostics)
			continue
		}

		got, err := attributeValue(readResp.State.Raw, liveBindingsPath)

		if err != nil {
			test.Fatal(err)
		}

		want, err := attributeValue(wantState, liveBindingsPath)

		if err != nil {
			test.Fatal(err)
		}

		if got.IsNull() || !got.Equal(want) {
			test.Errorf("%s: live_bindings = %v, want %v", name, got, want)
		}
	}
}

func TestResourceModifyPlanLiveBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var orders capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}
	var priorityOrders capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "orders", Priority: 2}, Target: "acmeCo/orders"}
	var users capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "users"}, Target: "acmeCo/users"}
	var events capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "events"}, Target: "acmeCo/events"}
	var stored []capture.Binding[testResource] = []capture.Binding[testResource]{events, orders}
	var drifted []capture.Binding[testResource] = []capture.Binding[testResource]{events, {Resource: testResource{Table: "orders", Priority: 9}, Target: "acmeCo/orders"}}

	expectations := map[string]struct {
		create       bool
		autoDiscover *capture.AutoDiscover
		stored       []capture.Binding[testResource]
		planned      []capture.Binding[testResource]
		unknownHost  bool
		unknown      bool
		want         []capture.Binding[testResource]
	}{
		"keeps the stored live bindings while bindings are unchanged": {false, nil, drifted, []capture.Binding[testResource]{orders}, false, false, drifted},
		"appends an added binding":                                    {false, nil, stored, []capture.Binding[testResource]{orders, users}, false, false, []capture.Binding[testResource]{events, orders, users}},
		"drops a removed binding and keeps the unmanaged ones":        {false, nil, stored, []capture.Binding[testResource]{}, false, false, []capture.Binding[testResource]{events}},
		"updates a changed binding in place":                          {false, nil, stored, []capture.Binding[testResource]{priorityOrders}, false, false, []capture.Binding[testResource]{events, priorityOrders}},
		"plans unknown while stored live bindings aren't known":       {false, nil, nil, []capture.Binding[testResource]{orders, users}, false, true, nil},
		"plans unknown while the plan holds unknown values":           {false, nil, stored, []capture.Binding[testResource]{orders, users}, true, true, nil},
		"plans the planned bindings when creating":                    {true, nil, nil, []capture.Binding[testResource]{orders}, false, false, []capture.Binding[testResource]{orders}},
		"plans unknown when creating with discovery":                  {true, &capture.AutoDiscover{AddNewBindings: true}, nil, []capture.Binding[testResource]{orders}, false, true, nil},
	}

	for name, expected := range expectations {
		var prior CaptureSpec[testConfig, testResource] = testCaptureSpec
		var listed CaptureSpec[testConfig, testResource] = testCaptureSpec
		var planned CaptureSpec[testConfig, testResource] = testCaptureSpec
		var wanted CaptureSpec[testConfig, testResource] = testCaptureSpec

		entry := newTestCaptureEntry(&testBackend{live: liveCapture()})

		prior.Bindings = []capture.Binding[testResource]{orders}
		listed.Bindings = expected.stored
		planned.AutoDiscover = expected.autoDiscover
		planned.Bindings = expected.planned
		wanted.Bindings = expected.want

		priorValue, err := entry.codec.Value("movies", prior)

		if err != nil {
			test.Fatal(err)
		}

		listedValue, err := entry.codec.Value("movies", listed)

		if err != nil {
			test.Fatal(err)
		}

		priorValue, err = withLiveBindings(priorValue, listedValue)

		if err == nil && expected.stored == nil {
			priorValue, err = setLiveBindings(priorValue, func(current tftypes.Value) tftypes.Value {
				return tftypes.NewValue(current.Type(), tftypes.UnknownValue)
			})
		}

		if err != nil {
			test.Fatal(err)
		}

		planValue, err := entry.codec.Value("movies", planned)

		if err == nil && expected.unknownHost {
			planValue, err = withValue(planValue, tftypes.NewAttributePath().WithAttributeName("config").WithAttributeName("host"), tftypes.UnknownValue)
		}

		if err != nil {
			test.Fatal(err)
		}

		wantedValue, err := entry.codec.Value("movies", wanted)

		if err != nil {
			test.Fatal(err)
		}

		wantPlan, err := withLiveBindings(planValue, wantedValue)

		if err != nil {
			test.Fatal(err)
		}

		if expected.create {
			priorValue = tftypes.NewValue(entry.codec.valueType, nil)
		}

		var modifyPlanReq resource.ModifyPlanRequest = resource.ModifyPlanRequest{
			State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: priorValue},
			Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
			Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: planValue},
		}
		var modifyPlanResp resource.ModifyPlanResponse = resource.ModifyPlanResponse{Plan: modifyPlanReq.Plan}

		entry.ModifyPlan(ctx, modifyPlanReq, &modifyPlanResp)

		if modifyPlanResp.Diagnostics.HasError() {
			test.Errorf("%s: modify plan: %v", name, modifyPlanResp.Diagnostics)
			continue
		}

		got, err := attributeValue(modifyPlanResp.Plan.Raw, liveBindingsPath)

		if err != nil {
			test.Fatal(err)
		}

		want, err := attributeValue(wantPlan, liveBindingsPath)

		if err != nil {
			test.Fatal(err)
		}

		if expected.unknown && got.IsKnown() {
			test.Errorf("%s: live_bindings = %v, want unknown", name, got)
		}

		if !expected.unknown && !got.Equal(want) {
			test.Errorf("%s: live_bindings = %v, want %v", name, got, want)
		}
	}
}

func TestResourceApplySetsLiveBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var orders capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}
	var events capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "events"}, Target: "acmeCo/events"}
	var planned CaptureSpec[testConfig, testResource] = testCaptureSpec
	var listed CaptureSpec[testConfig, testResource] = testCaptureSpec
	var live map[string]any = liveCapture(map[string]any{"resource": map[string]any{"table": "events"}, "target": "acmeCo/events"}, map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"})

	planned.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true}
	planned.Bindings = []capture.Binding[testResource]{orders}
	listed.Bindings = []capture.Binding[testResource]{events, orders}

	expectations := map[string]struct {
		live    map[string]any
		unknown bool
		reads   int
		warns   bool
		want    CaptureSpec[testConfig, testResource]
	}{
		"reads the live bindings the plan left unknown": {live, true, 1, false, listed},
		"keeps the planned live bindings":               {live, false, 0, false, planned},
		"warns when the entry can't be read":            {nil, true, 1, true, planned},
	}

	for name, expected := range expectations {
		var backend *testBackend = &testBackend{live: expected.live}

		entry := newTestCaptureEntry(backend)

		planValue, err := entry.codec.Value("movies", planned)

		if err != nil {
			test.Fatal(err)
		}

		planValue, err = withLiveBindings(planValue, planValue)

		if err == nil && expected.unknown {
			planValue, err = setLiveBindings(planValue, func(current tftypes.Value) tftypes.Value {
				return tftypes.NewValue(current.Type(), tftypes.UnknownValue)
			})
		}

		if err != nil {
			test.Fatal(err)
		}

		wantValue, err := entry.codec.Value("movies", expected.want)

		if err != nil {
			test.Fatal(err)
		}

		wantState, err := withLiveBindings(planValue, wantValue)

		if err == nil && expected.warns {
			wantState, err = setLiveBindings(planValue, func(current tftypes.Value) tftypes.Value {
				return tftypes.NewValue(current.Type(), nil)
			})
		}

		if err != nil {
			test.Fatal(err)
		}

		var createResp resource.CreateResponse = resource.CreateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

		entry.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue}}, &createResp)

		if createResp.Diagnostics.HasError() {
			test.Errorf("%s: create: %v", name, createResp.Diagnostics)
			continue
		}

		if len(backend.reads) != expected.reads || (createResp.Diagnostics.WarningsCount() > 0) != expected.warns {
			test.Errorf("%s: read %v with warnings %v, want %d reads and warnings %v", name, backend.reads, createResp.Diagnostics.Warnings(), expected.reads, expected.warns)
		}

		got, err := attributeValue(createResp.State.Raw, liveBindingsPath)

		if err != nil {
			test.Fatal(err)
		}

		want, err := attributeValue(wantState, liveBindingsPath)

		if err != nil {
			test.Fatal(err)
		}

		if !got.Equal(want) {
			test.Errorf("%s: live_bindings = %v, want %v", name, got, want)
		}
	}
}

func TestResourceImportStateListsLiveBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var backend *testBackend = &testBackend{live: liveCapture(map[string]any{"resource": map[string]any{"table": "events"}, "target": "acmeCo/events"})}
	var managed []tftypes.Value
	var live []tftypes.Value

	backend.live["autoDiscover"] = map[string]any{"addNewBindings": true}

	entry := newTestCaptureEntry(backend)

	var importResp resource.ImportStateResponse = resource.ImportStateResponse{
		State: tfsdk.State{
			Schema: entry.codec.Schema(),
			Raw:    tftypes.NewValue(entry.codec.valueType, nil),
		},
	}

	entry.ImportState(ctx, resource.ImportStateRequest{ID: "acmeCo/movies/source-test"}, &importResp)

	if importResp.Diagnostics.HasError() {
		test.Fatalf("import: %v", importResp.Diagnostics)
	}

	bindings, err := attributeValue(importResp.State.Raw, tftypes.NewAttributePath().WithAttributeName(bindingsAttribute))

	if err == nil && !bindings.IsNull() {
		err = bindings.As(&managed)
	}

	if err != nil {
		test.Fatal(err)
	}

	liveBindings, err := attributeValue(importResp.State.Raw, liveBindingsPath)

	if err == nil {
		err = liveBindings.As(&live)
	}

	if err != nil {
		test.Fatal(err)
	}

	if len(managed) != 0 || len(live) != 1 || bindingCollectionValue(live[0]) != "acmeCo/events" {
		test.Errorf("bindings = %v, live_bindings = %v, want only the discovered events binding, in live_bindings", managed, live)
	}
}

func TestMaterializationModifyPlanAcceptsUnknownBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var configureResp resource.ConfigureResponse
	var spec MaterializationSpec[testConfig, testResource] = MaterializationSpec[testConfig, testResource]{
		Endpoint: endpoint.ConnectorSpec[testConfig]{Connector: endpoint.Connector[testConfig]{Image: testNamedConnector.Image, Config: testConfig{Host: "db.example.com"}}},
		Bindings: []materialization.Binding[testResource]{{Resource: testResource{Table: "orders"}, Source: materialization.BindingSource{Name: "acmeCo/orders"}}},
	}

	entry := NewMaterialization[testConfig, testResource](testNamedConnector)().(*entryResource[materializationDocument[testConfig, testResource], MaterializationSpec[testConfig, testResource]])

	entry.Configure(ctx, resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: &testBackend{}, Tenant: "acmeCo/"}}, &configureResp)

	priorValue, err := entry.codec.Value("warehouse", spec)

	if err != nil {
		test.Fatal(err)
	}

	planValue, err := withValue(priorValue, tftypes.NewAttributePath().WithAttributeName(bindingsAttribute), tftypes.UnknownValue)

	if err != nil {
		test.Fatal(err)
	}

	expectations := map[string]tftypes.Value{
		"on create": tftypes.NewValue(entry.codec.valueType, nil),
		"on update": priorValue,
	}

	for name, state := range expectations {
		var modifyPlanReq resource.ModifyPlanRequest = resource.ModifyPlanRequest{
			State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: state},
			Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
			Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: planValue},
		}
		var modifyPlanResp resource.ModifyPlanResponse = resource.ModifyPlanResponse{Plan: modifyPlanReq.Plan}

		entry.ModifyPlan(ctx, modifyPlanReq, &modifyPlanResp)

		if modifyPlanResp.Diagnostics.HasError() {
			test.Errorf("%s: modify plan: %v", name, modifyPlanResp.Diagnostics)
			continue
		}

		bindings, err := attributeValue(modifyPlanResp.Plan.Raw, tftypes.NewAttributePath().WithAttributeName(bindingsAttribute))

		if err != nil {
			test.Fatal(err)
		}

		if bindings.IsKnown() {
			test.Errorf("%s: bindings = %v, want them left unknown", name, bindings)
		}
	}
}
