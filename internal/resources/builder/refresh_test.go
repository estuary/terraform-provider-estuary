package builder

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func liveCapture(bindings ...any) map[string]any {
	return map[string]any{
		"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com"}}},
		"bindings": bindings,
	}
}

func TestResourceReadRefreshesManagedBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var discovering CaptureSpec[testConfig, testResource] = testCaptureSpec
	var plain CaptureSpec[testConfig, testResource] = testCaptureSpec
	var users capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "users", Priority: 2}, Target: "acmeCo/users"}
	var orders capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}
	var events map[string]any = map[string]any{"resource": map[string]any{"table": "events"}, "target": "acmeCo/events"}

	discovering.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true}

	expectations := map[string]struct {
		state    CaptureSpec[testConfig, testResource]
		bindings []capture.Binding[testResource]
		live     []any
		want     []capture.Binding[testResource]
	}{
		"refreshes a field changed outside Terraform": {
			discovering, []capture.Binding[testResource]{users},
			[]any{map[string]any{"resource": map[string]any{"table": "users", "priority": 3, "_meta": map[string]any{"path": []any{"users"}}}, "target": "acmeCo/users"}},
			[]capture.Binding[testResource]{{Resource: testResource{Table: "users", Priority: 3}, Target: "acmeCo/users"}},
		},
		"keeps the fields Estuary fills in unmanaged": {
			discovering, []capture.Binding[testResource]{orders},
			[]any{map[string]any{"resource": map[string]any{"table": "orders", "priority": 5}, "target": "acmeCo/orders", "backfill": 2}},
			[]capture.Binding[testResource]{orders},
		},
		"drops a binding removed outside Terraform": {
			discovering, []capture.Binding[testResource]{users, orders},
			[]any{map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"}},
			[]capture.Binding[testResource]{orders},
		},
		"leaves the discovered bindings unmanaged": {
			discovering, []capture.Binding[testResource]{orders},
			[]any{events, map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"}},
			[]capture.Binding[testResource]{orders},
		},
		"adds the live bindings Terraform removes without auto discover": {
			plain, []capture.Binding[testResource]{orders},
			[]any{events, map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"}},
			[]capture.Binding[testResource]{orders, {Resource: testResource{Table: "events"}, Target: "acmeCo/events"}},
		},
	}

	for name, expected := range expectations {
		var backend *testBackend = &testBackend{live: liveCapture(expected.live...)}
		var state CaptureSpec[testConfig, testResource] = expected.state
		var refreshed CaptureSpec[testConfig, testResource] = expected.state

		entry := newTestCaptureEntry(backend)

		state.Bindings = expected.bindings
		refreshed.Bindings = expected.want

		stateValue, err := entry.codec.Value("movies", state)

		if err != nil {
			test.Fatal(err)
		}

		want, err := entry.codec.Value("movies", refreshed)

		if err != nil {
			test.Fatal(err)
		}

		var readResp resource.ReadResponse = resource.ReadResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}}

		entry.Read(ctx, resource.ReadRequest{State: readResp.State}, &readResp)

		if readResp.Diagnostics.HasError() || readResp.Diagnostics.WarningsCount() > 0 {
			test.Errorf("%s: read: %v", name, readResp.Diagnostics)
			continue
		}

		if !readResp.State.Raw.Equal(want) {
			test.Errorf("%s: refreshed state = %v, want %v", name, readResp.State.Raw, want)
		}
	}
}

func TestResourceReadKeepsZeroValues(test *testing.T) {
	var spec CaptureSpec[testConfig, testResource] = testCaptureSpec

	entry := newTestCaptureEntry(&testBackend{live: liveCapture(map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"})})

	spec.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}

	stateValue, err := entry.codec.Value("movies", spec)

	if err == nil {
		stateValue, err = withValue(stateValue, tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(0).WithAttributeName("backfill"), 0)
	}

	if err != nil {
		test.Fatal(err)
	}

	var readResp resource.ReadResponse = resource.ReadResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}}

	entry.Read(context.Background(), resource.ReadRequest{State: readResp.State}, &readResp)

	if readResp.Diagnostics.HasError() || !readResp.State.Raw.Equal(stateValue) {
		test.Errorf("refreshed state = %v (%v), want backfill = 0 kept as the live spec omits it", readResp.State.Raw, readResp.Diagnostics)
	}
}

func TestResourceReadLiveEntry(test *testing.T) {
	var ctx context.Context = context.Background()

	expectations := map[string]struct {
		live    map[string]any
		removed bool
		warn    bool
	}{
		"removes a deleted entry":         {live: nil, removed: true},
		"warns about an undecodable spec": {live: map[string]any{"bindings": "not a list"}, warn: true},
		"keeps an entry without bindings": {live: liveCapture()},
	}

	for name, expected := range expectations {
		entry := newTestCaptureEntry(&testBackend{live: expected.live})

		stateValue, err := entry.codec.Value("movies", testCaptureSpec)

		if err != nil {
			test.Fatal(err)
		}

		var readResp resource.ReadResponse = resource.ReadResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}}

		entry.Read(ctx, resource.ReadRequest{State: readResp.State}, &readResp)

		if readResp.Diagnostics.HasError() {
			test.Errorf("%s: read: %v", name, readResp.Diagnostics)
			continue
		}

		if removed := readResp.State.Raw.IsNull(); removed != expected.removed {
			test.Errorf("%s: removed = %v, want %v", name, removed, expected.removed)
		}

		if warned := readResp.Diagnostics.WarningsCount() > 0; warned != expected.warn {
			test.Errorf("%s: warned = %v, want %v", name, warned, expected.warn)
		}

		if !expected.removed && !readResp.State.Raw.Equal(stateValue) {
			test.Errorf("%s: state = %v, want it unchanged", name, readResp.State.Raw)
		}
	}
}

func TestResourceModifyPlanWarnsAboutTakenOverBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var prior CaptureSpec[testConfig, testResource] = testCaptureSpec
	var planned CaptureSpec[testConfig, testResource] = testCaptureSpec
	var backend *testBackend = &testBackend{live: liveCapture(
		map[string]any{"resource": map[string]any{"table": "users"}, "target": "acmeCo/users"},
		map[string]any{"resource": map[string]any{"table": "logs"}, "target": "acmeCo/logs"},
	)}

	entry := newTestCaptureEntry(backend)

	prior.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true}
	prior.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "logs"}, Target: "acmeCo/logs"}}

	planned.AutoDiscover = prior.AutoDiscover
	planned.Bindings = []capture.Binding[testResource]{
		{Resource: testResource{Table: "logs", Priority: 1}, Target: "acmeCo/logs"},
		{Resource: testResource{Table: "users"}, Target: "acmeCo/users"},
		{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"},
	}

	priorValue, err := entry.codec.Value("movies", prior)

	if err != nil {
		test.Fatal(err)
	}

	planValue, err := entry.codec.Value("movies", planned)

	if err != nil {
		test.Fatal(err)
	}

	var modifyPlanReq resource.ModifyPlanRequest = resource.ModifyPlanRequest{
		State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: priorValue},
		Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
		Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: planValue},
	}
	var modifyPlanResp resource.ModifyPlanResponse = resource.ModifyPlanResponse{Plan: modifyPlanReq.Plan}

	entry.ModifyPlan(ctx, modifyPlanReq, &modifyPlanResp)

	if modifyPlanResp.Diagnostics.HasError() {
		test.Fatalf("modify plan: %v", modifyPlanResp.Diagnostics)
	}

	var warnings = modifyPlanResp.Diagnostics.Warnings()

	if len(warnings) != 1 {
		test.Fatalf("warnings = %v, want one for the taken over users binding", warnings)
	}

	if warning, isAttribute := warnings[0].(interface{ Path() path.Path }); !isAttribute || !warning.Path().Equal(path.Root("bindings").AtListIndex(1)) {
		test.Errorf("warning = %v, want it at bindings[1]", warnings[0])
	}

	if !strings.Contains(warnings[0].Detail(), "No field changes") {
		test.Errorf("warning detail = %q, want no field changes for the same live binding", warnings[0].Detail())
	}

	backend.reads = nil
	modifyPlanReq.Plan.Raw = priorValue
	modifyPlanReq.Config.Raw = priorValue

	entry.ModifyPlan(ctx, modifyPlanReq, &modifyPlanResp)

	if len(backend.reads) != 0 {
		test.Errorf("read specs of %v, want none without a binding to take over", backend.reads)
	}
}

func TestResourceModifyPlanWarnsAboutRemovedBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var users capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "users"}, Target: "acmeCo/users"}
	var logs capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "logs"}, Target: "acmeCo/logs"}

	expectations := map[string]struct {
		autoDiscover *capture.AutoDiscover
		planned      []capture.Binding[testResource]
		want         int
	}{
		"warns when discovery adds new bindings":   {&capture.AutoDiscover{AddNewBindings: true}, []capture.Binding[testResource]{logs}, 1},
		"keeps a disabled binding without warning": {&capture.AutoDiscover{AddNewBindings: true}, []capture.Binding[testResource]{logs, {Resource: users.Resource, Target: users.Target, Disable: true}}, 0},
		"skips discovery not adding new bindings":  {&capture.AutoDiscover{}, []capture.Binding[testResource]{logs}, 0},
		"skips captures without auto discover":     {nil, []capture.Binding[testResource]{logs}, 0},
		"warns about every removed binding":        {&capture.AutoDiscover{AddNewBindings: true}, []capture.Binding[testResource]{}, 2},
	}

	for name, expected := range expectations {
		var prior CaptureSpec[testConfig, testResource] = testCaptureSpec
		var planned CaptureSpec[testConfig, testResource] = testCaptureSpec

		entry := newTestCaptureEntry(&testBackend{live: liveCapture()})

		prior.AutoDiscover = expected.autoDiscover
		prior.Bindings = []capture.Binding[testResource]{users, logs}

		planned.AutoDiscover = expected.autoDiscover
		planned.Bindings = expected.planned

		priorValue, err := entry.codec.Value("movies", prior)

		if err != nil {
			test.Fatal(err)
		}

		planValue, err := entry.codec.Value("movies", planned)

		if err != nil {
			test.Fatal(err)
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

		var warnings = modifyPlanResp.Diagnostics.Warnings()

		if len(warnings) != expected.want {
			test.Errorf("%s: warnings = %v, want %d", name, warnings, expected.want)
			continue
		}

		for _, warning := range warnings {
			if !strings.Contains(warning.Detail(), "disable = true") {
				test.Errorf("%s: warning detail = %q, want the disable advice", name, warning.Detail())
			}
		}
	}
}

func TestCodecBindingChanges(test *testing.T) {
	var path *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(0)
	var users capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "users"}, Target: "acmeCo/users"}

	codec, err := NewCaptureCodec[testConfig, testResource](testNamedConnector)

	if err != nil {
		test.Fatal(err)
	}

	expectations := map[string]struct {
		planned capture.Binding[testResource]
		live    capture.Binding[testResource]
		want    []string
	}{
		"same binding":            {users, users, nil},
		"adds a backfill":         {capture.Binding[testResource]{Resource: users.Resource, Target: users.Target, Backfill: 1}, users, []string{"bindings[0].backfill: (unset) -> 1"}},
		"drops a live-only field": {users, capture.Binding[testResource]{Resource: testResource{Table: "users", Priority: 5}, Target: users.Target}, []string{"bindings[0].resource.priority: 5 -> (unset)"}},
		"changes several fields": {
			capture.Binding[testResource]{Resource: testResource{Table: "people", Priority: 1}, Target: users.Target, Disable: true},
			users,
			[]string{"bindings[0].disable: (unset) -> true", "bindings[0].resource.priority: (unset) -> 1", `bindings[0].resource.table: "users" -> "people"`},
		},
	}

	for name, expected := range expectations {
		var plannedSpec, liveSpec CaptureSpec[testConfig, testResource] = testCaptureSpec, testCaptureSpec

		plannedSpec.Bindings = []capture.Binding[testResource]{expected.planned}
		liveSpec.Bindings = []capture.Binding[testResource]{expected.live}

		plannedValue, err := codec.Value("movies", plannedSpec)

		if err != nil {
			test.Fatal(err)
		}

		liveValue, err := codec.Value("movies", liveSpec)

		if err != nil {
			test.Fatal(err)
		}

		planned, _, _ := bindings(plannedValue)
		live, _, _ := bindings(liveValue)

		changes, err := codec.BindingChanges(path, planned[0], live[0])

		if err != nil || !slices.Equal(changes, expected.want) {
			test.Errorf("%s: changes = %q (%v), want %q", name, changes, err, expected.want)
		}
	}
}
