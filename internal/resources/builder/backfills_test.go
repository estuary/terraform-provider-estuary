package builder

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/postgres"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var testBackfillBlockPath *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(0).WithAttributeName("backfill")
var testCaptureBackfillPath *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName("backfill")

func TestResourceUpdateBackfillTriggers(test *testing.T) {
	var ctx context.Context = context.Background()

	expectations := map[string]struct {
		prior     any
		planned   any
		strategy  any
		managed   bool
		published any
		resets    []string
	}{
		"raises the live counter":                        {1, 2, nil, true, 5, nil},
		"raises the live counter when set":               {nil, 1, "incremental", true, 5, nil},
		"publishes nothing without a raise":              {2, 2, nil, true, nil, nil},
		"publishes nothing when removed":                 {2, nil, nil, true, nil, nil},
		"resets with the live counter":                   {1, 2, "dataflowReset", true, 4, []string{"acmeCo/orders"}},
		"backfills a binding taken over with a trigger":  {nil, 1, "incremental", false, 5, nil},
		"resets a binding taken over with a trigger":     {nil, 3, "dataflowReset", false, 4, []string{"acmeCo/orders"}},
		"keeps the live counter of a binding taken over": {nil, nil, nil, false, nil, nil},
	}

	for name, expected := range expectations {
		var backend *testBackend = &testBackend{live: liveCapture(map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders", "backfill": 4})}
		var prior CaptureSpec[testConfig, testResource] = testCaptureSpec
		var planned CaptureSpec[testConfig, testResource] = testCaptureSpec

		entry := newTestCaptureEntry(backend)

		planned.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}

		if expected.managed {
			prior.Bindings = planned.Bindings
		}

		priorValue, err := entry.codec.Value("movies", prior)

		if err == nil && expected.managed {
			priorValue, err = withValue(priorValue, testBackfillBlockPath, map[string]tftypes.Value{
				"trigger":  tftypes.NewValue(tftypes.Number, expected.prior),
				"strategy": tftypes.NewValue(tftypes.String, expected.strategy),
			})
		}

		if err != nil {
			test.Fatal(err)
		}

		planValue, err := entry.codec.Value("movies", planned)

		if err == nil {
			planValue, err = withValue(planValue, testBackfillBlockPath, map[string]tftypes.Value{
				"trigger":  tftypes.NewValue(tftypes.Number, expected.planned),
				"strategy": tftypes.NewValue(tftypes.String, expected.strategy),
			})
		}

		if err != nil {
			test.Fatal(err)
		}

		var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

		entry.Update(ctx, resource.UpdateRequest{
			State: tfsdk.State{Schema: entry.codec.Schema(), Raw: priorValue},
			Plan:  tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
		}, &updateResp)

		if updateResp.Diagnostics.HasError() {
			test.Errorf("%s: update: %v", name, updateResp.Diagnostics)
			continue
		}

		if expected.published == nil && backend.published != nil {
			test.Errorf("%s: published %v, want nothing published as the live spec is unchanged", name, backend.published)
		}

		bindings, _ := backend.published["bindings"].([]any)

		if expected.published != nil && (len(bindings) != 1 || bindings[0].(map[string]any)["backfill"] != expected.published || !reflect.DeepEqual(backend.resets, expected.resets)) {
			test.Errorf("%s: published bindings %v and reset %v, want backfill %v and %v", name, bindings, backend.resets, expected.published, expected.resets)
		}

		if !updateResp.State.Raw.Equal(planValue) {
			test.Errorf("%s: state differs from the plan, want the planned backfill block", name)
		}

		warnings := entry.warnDataflowResets(planValue, priorValue, entry.newLiveReader(ctx, priorValue))

		if warned := len(warnings) == 1 && strings.Contains(warnings[0].Detail(), "- acmeCo/orders"); warned != (expected.resets != nil) {
			test.Errorf("%s: warnings = %v, want a dataflow reset warning %v", name, warnings, expected.resets != nil)
		}
	}
}

func TestBackfillRequestsOfUnmanagedBindings(test *testing.T) {
	expectations := map[string]struct {
		live      []string
		backfills []string
	}{
		"backfills a live binding taken over": {[]string{"acmeCo/orders"}, []string{"acmeCo/orders"}},
		"skips a binding not live yet":        {nil, nil},
	}

	entry := newTestCaptureEntry(&testBackend{})

	var planned CaptureSpec[testConfig, testResource] = testCaptureSpec

	planned.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}

	priorValue, err := entry.codec.Value("movies", testCaptureSpec)

	if err != nil {
		test.Fatal(err)
	}

	planValue, err := entry.codec.Value("movies", planned)

	if err == nil {
		planValue, err = withValue(planValue, testBackfillBlockPath, map[string]tftypes.Value{
			"trigger":  tftypes.NewValue(tftypes.Number, 1),
			"strategy": tftypes.NewValue(tftypes.String, nil),
		})
	}

	if err != nil {
		test.Fatal(err)
	}

	if !triggersUnmanaged(planValue, priorValue) {
		test.Errorf("triggersUnmanaged = false, want true for a trigger on a binding the prior state doesn't hold")
	}

	for name, expected := range expectations {
		backfills, resets, err := backfillRequests(planValue, priorValue, expected.live)

		if err != nil {
			test.Fatalf("%s: %v", name, err)
		}

		if !reflect.DeepEqual(backfills, expected.backfills) || resets != nil {
			test.Errorf("%s: backfills %v and resets %v, want %v and none", name, backfills, resets, expected.backfills)
		}
	}
}

func TestValidateBackfillTriggers(test *testing.T) {
	expectations := map[string]struct {
		prior   any
		planned any
		summary string
	}{
		"accepts a raise":            {2, 3, ""},
		"accepts an unchanged value": {2, 2, ""},
		"accepts a removal":          {2, nil, ""},
		"rejects a lowered value":    {3, 2, "Backfill trigger lowered"},
		"rejects zero":               {nil, 0, "Invalid backfill trigger"},
		"rejects an unknown value":   {2, tftypes.UnknownValue, "Backfill trigger unknown at plan time"},
	}

	entry := newTestCaptureEntry(&testBackend{})

	for name, expected := range expectations {
		var spec CaptureSpec[testConfig, testResource] = testCaptureSpec

		spec.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}

		value, err := entry.codec.Value("movies", spec)

		if err != nil {
			test.Fatal(err)
		}

		priorValue, err := withValue(value, testBackfillBlockPath, map[string]tftypes.Value{
			"trigger":  tftypes.NewValue(tftypes.Number, expected.prior),
			"strategy": tftypes.NewValue(tftypes.String, nil),
		})

		if err != nil {
			test.Fatal(err)
		}

		planValue, err := withValue(value, testBackfillBlockPath, map[string]tftypes.Value{
			"trigger":  tftypes.NewValue(tftypes.Number, expected.planned),
			"strategy": tftypes.NewValue(tftypes.String, nil),
		})

		if err != nil {
			test.Fatal(err)
		}

		diagnostics := validateBackfillTriggers(planValue, priorValue)

		if expected.summary == "" && diagnostics.HasError() {
			test.Errorf("%s: errors = %v, want none", name, diagnostics)
		}

		if expected.summary != "" && (len(diagnostics) != 1 || diagnostics[0].Summary() != expected.summary) {
			test.Errorf("%s: errors = %v, want %q", name, diagnostics, expected.summary)
		}
	}
}

func TestResourceReadKeepsBackfillSettings(test *testing.T) {
	var ctx context.Context = context.Background()
	var backend *testBackend = &testBackend{live: liveCapture(map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders", "backfill": 2})}
	var state CaptureSpec[testConfig, testResource] = testCaptureSpec

	entry := newTestCaptureEntry(backend)

	state.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}

	stateValue, err := entry.codec.Value("movies", state)

	if err == nil {
		stateValue, err = withValue(stateValue, testBackfillBlockPath, map[string]tftypes.Value{
			"trigger":  tftypes.NewValue(tftypes.Number, 1),
			"strategy": tftypes.NewValue(tftypes.String, "dataflowReset"),
		})
	}

	if err != nil {
		test.Fatal(err)
	}

	var readResp resource.ReadResponse = resource.ReadResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}}

	entry.Read(ctx, resource.ReadRequest{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}}, &readResp)

	if readResp.Diagnostics.HasError() {
		test.Fatalf("read: %v", readResp.Diagnostics)
	}

	if !readResp.State.Raw.Equal(stateValue) {
		test.Errorf("refreshed state = %v, want the backfill block kept", readResp.State.Raw)
	}
}

func TestResourceUpdateCaptureBackfillTrigger(test *testing.T) {
	var ctx context.Context = context.Background()

	expectations := map[string]struct {
		strategy any
		override any
		orders   int
		events   int
		resets   []string
	}{
		"backfills every binding":                {nil, nil, 5, 2, nil},
		"resets every binding":                   {"dataflowReset", nil, 4, 1, []string{"acmeCo/events", "acmeCo/orders"}},
		"resets the bindings without a override": {"dataflowReset", "incremental", 5, 1, []string{"acmeCo/events"}},
		"backfills the bindings without a reset": {"incremental", "dataflowReset", 4, 2, []string{"acmeCo/orders"}},
	}

	for name, expected := range expectations {
		var backend *testBackend = &testBackend{live: liveCapture(
			map[string]any{"resource": map[string]any{"table": "events"}, "target": "acmeCo/events", "backfill": 1},
			map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders", "backfill": 4},
		)}
		var spec CaptureSpec[testConfig, testResource] = testCaptureSpec

		entry := newTestCaptureEntry(backend)

		backend.live["autoDiscover"] = map[string]any{"addNewBindings": true}

		spec.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true}
		spec.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}

		value, err := entry.codec.Value("movies", spec)

		if err != nil {
			test.Fatal(err)
		}

		priorValue, err := withValue(value, testCaptureBackfillPath, map[string]tftypes.Value{
			"trigger":  tftypes.NewValue(tftypes.Number, 1),
			"strategy": tftypes.NewValue(tftypes.String, nil),
		})

		if err != nil {
			test.Fatal(err)
		}

		planValue, err := withValue(value, testCaptureBackfillPath, map[string]tftypes.Value{
			"trigger":  tftypes.NewValue(tftypes.Number, 2),
			"strategy": tftypes.NewValue(tftypes.String, expected.strategy),
		})

		if err == nil {
			planValue, err = withValue(planValue, testBackfillBlockPath, map[string]tftypes.Value{
				"trigger":  tftypes.NewValue(tftypes.Number, nil),
				"strategy": tftypes.NewValue(tftypes.String, expected.override),
			})
		}

		if err != nil {
			test.Fatal(err)
		}

		var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

		entry.Update(ctx, resource.UpdateRequest{
			State: tfsdk.State{Schema: entry.codec.Schema(), Raw: priorValue},
			Plan:  tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
		}, &updateResp)

		if updateResp.Diagnostics.HasError() {
			test.Errorf("%s: update: %v", name, updateResp.Diagnostics)
			continue
		}

		var published map[string]any = map[string]any{}

		bindings, _ := backend.published["bindings"].([]any)

		for _, binding := range bindings {
			object, _ := binding.(map[string]any)

			published[bindingCollection(binding)] = object["backfill"]
		}

		if published["acmeCo/orders"] != expected.orders || published["acmeCo/events"] != expected.events || !reflect.DeepEqual(backend.resets, expected.resets) {
			test.Errorf("%s: published backfills %v and reset %v, want orders %d, events %d and %v", name, published, backend.resets, expected.orders, expected.events, expected.resets)
		}

		warnings := entry.warnDataflowResets(planValue, priorValue, entry.newLiveReader(ctx, priorValue))

		if warned := len(warnings) == 1 && strings.Contains(warnings[0].Detail(), "- "+strings.Join(expected.resets, "\n- ")); warned != (expected.resets != nil) {
			test.Errorf("%s: warnings = %v, want a dataflow reset warning listing %v", name, warnings, expected.resets)
		}
	}
}

func TestValidateCaptureBackfillTrigger(test *testing.T) {
	entry := newTestCaptureEntry(&testBackend{})

	value, err := entry.codec.Value("movies", testCaptureSpec)

	if err != nil {
		test.Fatal(err)
	}

	priorValue, err := withValue(value, testCaptureBackfillPath, map[string]tftypes.Value{
		"trigger":  tftypes.NewValue(tftypes.Number, 3),
		"strategy": tftypes.NewValue(tftypes.String, nil),
	})

	if err != nil {
		test.Fatal(err)
	}

	planValue, err := withValue(value, testCaptureBackfillPath, map[string]tftypes.Value{
		"trigger":  tftypes.NewValue(tftypes.Number, 2),
		"strategy": tftypes.NewValue(tftypes.String, nil),
	})

	if err != nil {
		test.Fatal(err)
	}

	diagnostics := validateBackfillTriggers(planValue, priorValue)

	if len(diagnostics) != 1 || diagnostics[0].Summary() != "Backfill trigger lowered" || !strings.Contains(diagnostics[0].Detail(), "the capture") {
		test.Errorf("errors = %v, want the lowered capture trigger rejected", diagnostics)
	}
}

func TestCaptureBackfillModeInBackfillBlock(test *testing.T) {
	var ctx context.Context = context.Background()
	var configureResp resource.ConfigureResponse
	var backend *testBackend = &testBackend{live: map[string]any{
		"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"address": "db:5432", "user": "flow", "database": "shop", "historyMode": false}}},
		"bindings": []any{map[string]any{"resource": map[string]any{"namespace": "public", "stream": "orders", "mode": "Normal"}, "target": "acmeCo/orders"}},
	}}

	entry := NewCapture[postgres.Config, postgres.Resource](testNamedConnector)().(*entryResource[captureDocument[postgres.Config, postgres.Resource], CaptureSpec[postgres.Config, postgres.Resource]])

	entry.Configure(ctx, resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/"}}, &configureResp)

	_, resourceDiagnostics := entry.codec.Schema().AttributeAtPath(ctx, path.Root("bindings").AtListIndex(0).AtName("resource").AtName("mode"))
	mode, blockDiagnostics := entry.codec.Schema().AttributeAtPath(ctx, path.Root("bindings").AtListIndex(0).AtName("backfill").AtName("mode"))

	if !resourceDiagnostics.HasError() || blockDiagnostics.HasError() || reflect.TypeOf(mode) != reflect.TypeOf(schema.StringAttribute{}) {
		test.Fatalf("mode = %T (%v), want it in the binding backfill block only", mode, blockDiagnostics)
	}

	var spec CaptureSpec[postgres.Config, postgres.Resource] = CaptureSpec[postgres.Config, postgres.Resource]{
		Endpoint: endpoint.ConnectorSpec[postgres.Config]{Connector: endpoint.Connector[postgres.Config]{
			Image:  testNamedConnector.Image,
			Config: postgres.Config{Address: "db:5432", User: "flow", Database: "shop"},
		}},
		Bindings: []capture.Binding[postgres.Resource]{{Resource: postgres.Resource{Namespace: "public", Stream: "orders", Mode: "Precise"}, Target: "acmeCo/orders"}},
	}

	value, err := entry.codec.Value("shop", spec)

	if err != nil {
		test.Fatal(err)
	}

	blockMode, err := attributeValue(value, testBackfillBlockPath.WithAttributeName("mode"))

	if err != nil || !blockMode.Equal(tftypes.NewValue(tftypes.String, "Precise")) {
		test.Errorf("backfill.mode = %v (%v), want the resource mode", blockMode, err)
	}

	_, roundTrip, err := entry.codec.Spec(value)

	if err != nil || roundTrip.Bindings[0].Resource.Mode != "Precise" {
		test.Errorf("round trip resource = %+v (%v), want its mode back in the resource", roundTrip.Bindings[0].Resource, err)
	}

	planValue, err := withValue(value, testBackfillBlockPath.WithAttributeName("trigger"), 1)

	if err != nil {
		test.Fatal(err)
	}

	var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

	entry.Update(ctx, resource.UpdateRequest{
		State: tfsdk.State{Schema: entry.codec.Schema(), Raw: value},
		Plan:  tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
	}, &updateResp)

	bindings, _ := backend.published["bindings"].([]any)

	if updateResp.Diagnostics.HasError() || len(bindings) != 1 || utils.Lookup(bindings[0], "resource")["mode"] != "Precise" {
		test.Errorf("published bindings = %v (%v), want the mode in the resource", bindings, updateResp.Diagnostics)
	}

}

func TestResourceUpdateCaptureBackfillTriggerWithoutManagedBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var spec CaptureSpec[testConfig, testResource] = testCaptureSpec
	var backend *testBackend = &testBackend{live: liveCapture(
		map[string]any{"resource": map[string]any{"table": "events"}, "target": "acmeCo/events", "backfill": 1},
		map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"},
	)}

	entry := newTestCaptureEntry(backend)

	backend.live["autoDiscover"] = map[string]any{"addNewBindings": true}

	spec.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true}

	value, err := entry.codec.Value("movies", spec)

	if err != nil {
		test.Fatal(err)
	}

	planValue, err := withValue(value, testCaptureBackfillPath, map[string]tftypes.Value{
		"trigger":  tftypes.NewValue(tftypes.Number, 1),
		"strategy": tftypes.NewValue(tftypes.String, nil),
	})

	if err != nil {
		test.Fatal(err)
	}

	var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

	entry.Update(ctx, resource.UpdateRequest{
		State: tfsdk.State{Schema: entry.codec.Schema(), Raw: value},
		Plan:  tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
	}, &updateResp)

	if updateResp.Diagnostics.HasError() {
		test.Fatalf("update: %v", updateResp.Diagnostics)
	}

	var published map[string]any = map[string]any{}

	bindings, _ := backend.published["bindings"].([]any)

	for _, binding := range bindings {
		object, _ := binding.(map[string]any)

		published[bindingCollection(binding)] = object["backfill"]
	}

	if !reflect.DeepEqual(published, map[string]any{"acmeCo/events": 2, "acmeCo/orders": 1}) {
		test.Errorf("published backfills = %v, want every unmanaged binding raised by one", published)
	}
}

func TestResourceUpdateRequiresBackfillsOfChangedModes(test *testing.T) {
	var ctx context.Context = context.Background()

	expectations := map[string]struct {
		managed  bool
		prior    any
		trigger  any
		capture  any
		rejected bool
		sales    any
	}{
		"rejects a taken over binding without a trigger": {false, nil, nil, nil, true, nil},
		"backfills a taken over binding with a trigger":  {false, nil, 1, nil, false, 4},
		"rejects a managed binding without a raise":      {true, 1, 1, nil, true, nil},
		"backfills a managed binding with a raise":       {true, 1, 2, nil, false, 4},
		"backfills a binding with the capture trigger":   {true, nil, nil, 1, false, 4},
	}

	for name, expected := range expectations {
		var configureResp resource.ConfigureResponse
		var backend *testBackend = &testBackend{live: map[string]any{
			"autoDiscover": map[string]any{"addNewBindings": true},
			"endpoint":     map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"address": "db:5432", "user": "flow", "database": "shop", "historyMode": false}}},
			"bindings": []any{
				map[string]any{"resource": map[string]any{"namespace": "public", "stream": "sales"}, "target": "acmeCo/sales", "backfill": 3},
				map[string]any{"resource": map[string]any{"namespace": "public", "stream": "users", "mode": "Precise"}, "target": "acmeCo/users"},
			},
		}}

		entry := NewCapture[postgres.Config, postgres.Resource](testNamedConnector)().(*entryResource[captureDocument[postgres.Config, postgres.Resource], CaptureSpec[postgres.Config, postgres.Resource]])

		entry.Configure(ctx, resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/"}}, &configureResp)

		var prior CaptureSpec[postgres.Config, postgres.Resource] = CaptureSpec[postgres.Config, postgres.Resource]{
			AutoDiscover: &capture.AutoDiscover{AddNewBindings: true},
			Endpoint: endpoint.ConnectorSpec[postgres.Config]{Connector: endpoint.Connector[postgres.Config]{
				Image:  testNamedConnector.Image,
				Config: postgres.Config{Address: "db:5432", User: "flow", Database: "shop"},
			}},
		}
		var planned CaptureSpec[postgres.Config, postgres.Resource] = prior

		planned.Bindings = []capture.Binding[postgres.Resource]{{Resource: postgres.Resource{Namespace: "public", Stream: "sales", Mode: "Normal"}, Target: "acmeCo/sales"}}

		if expected.managed {
			prior.Bindings = []capture.Binding[postgres.Resource]{{Resource: postgres.Resource{Namespace: "public", Stream: "sales"}, Target: "acmeCo/sales"}}
		}

		priorValue, err := entry.codec.Value("shop", prior)

		if err == nil && expected.managed {
			priorValue, err = withValue(priorValue, testBackfillBlockPath, map[string]tftypes.Value{
				"trigger":  tftypes.NewValue(tftypes.Number, expected.prior),
				"strategy": tftypes.NewValue(tftypes.String, nil),
				"mode":     tftypes.NewValue(tftypes.String, nil),
			})
		}

		if err == nil && expected.capture != nil {
			priorValue, err = withValue(priorValue, testCaptureBackfillPath, map[string]tftypes.Value{
				"trigger":  tftypes.NewValue(tftypes.Number, nil),
				"strategy": tftypes.NewValue(tftypes.String, nil),
			})
		}

		if err != nil {
			test.Fatal(err)
		}

		planValue, err := entry.codec.Value("shop", planned)

		if err == nil {
			planValue, err = withValue(planValue, testBackfillBlockPath.WithAttributeName("trigger"), expected.trigger)
		}

		if err == nil && expected.capture != nil {
			planValue, err = withValue(planValue, testCaptureBackfillPath, map[string]tftypes.Value{
				"trigger":  tftypes.NewValue(tftypes.Number, expected.capture),
				"strategy": tftypes.NewValue(tftypes.String, nil),
			})
		}

		if err != nil {
			test.Fatal(err)
		}

		diagnostics := entry.validateModeBackfills(planValue, priorValue, entry.newLiveReader(ctx, priorValue))

		if diagnostics.HasError() != expected.rejected || (expected.rejected && !strings.Contains(diagnostics[0].Detail(), `changes "" -> "Normal"`)) {
			test.Errorf("%s: plan diagnostics = %v, want rejected %v", name, diagnostics, expected.rejected)
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

		var published map[string]any = map[string]any{}

		bindings, _ := backend.published["bindings"].([]any)

		for _, binding := range bindings {
			object, _ := binding.(map[string]any)

			published[bindingCollection(binding)] = object["backfill"]
		}

		if published["acmeCo/sales"] != expected.sales || utils.Lookup(bindings[0], "resource")["mode"] != "Normal" {
			test.Errorf("%s: published bindings = %v, want the sales binding backfilled along with its mode", name, bindings)
		}
	}
}
