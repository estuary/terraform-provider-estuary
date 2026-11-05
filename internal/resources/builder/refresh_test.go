package builder

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// computedByRead reports the attributes Read and ImportState compute beside the refreshed ones, which tests check
// apart.
func computedByRead(path *tftypes.AttributePath) bool {
	return path.Equal(catalogNamePath)
}

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
		"refreshes a field only set live": {
			discovering, []capture.Binding[testResource]{orders},
			[]any{map[string]any{"resource": map[string]any{"table": "orders", "priority": 5}, "target": "acmeCo/orders"}},
			[]capture.Binding[testResource]{{Resource: testResource{Table: "orders", Priority: 5}, Target: "acmeCo/orders"}},
		},
		"leaves the empty live fields out": {
			discovering, []capture.Binding[testResource]{orders},
			[]any{map[string]any{"resource": map[string]any{"table": "orders", "priority": 0}, "target": "acmeCo/orders", "disable": false}},
			[]capture.Binding[testResource]{orders},
		},
		"refreshes the live backfill counter": {
			discovering, []capture.Binding[testResource]{orders},
			[]any{map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders", "backfill": 2}},
			[]capture.Binding[testResource]{{Resource: orders.Resource, Target: orders.Target, Backfill: 2}},
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
		"leaves the undeclared bindings unmanaged without auto discover": {
			plain, []capture.Binding[testResource]{orders},
			[]any{events, map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"}},
			[]capture.Binding[testResource]{orders},
		},
		"refreshes each binding of a shared collection from its own resource": {
			plain, []capture.Binding[testResource]{orders, {Resource: testResource{Table: "orders_archive"}, Target: "acmeCo/orders"}},
			[]any{
				map[string]any{"resource": map[string]any{"table": "orders_archive", "priority": 4}, "target": "acmeCo/orders"},
				map[string]any{"resource": map[string]any{"table": "orders", "priority": 1}, "target": "acmeCo/orders"},
			},
			[]capture.Binding[testResource]{
				{Resource: testResource{Table: "orders", Priority: 1}, Target: "acmeCo/orders"},
				{Resource: testResource{Table: "orders_archive", Priority: 4}, Target: "acmeCo/orders"},
			},
		},
	}

	for name, expected := range expectations {
		var backend *testBackend = &testBackend{live: liveCapture(expected.live...)}
		var state CaptureSpec[testConfig, testResource] = expected.state
		var refreshed CaptureSpec[testConfig, testResource] = expected.state

		entry := newTestCaptureEntry(backend)

		if expected.state.AutoDiscover != nil {
			backend.live["autoDiscover"] = map[string]any{"addNewBindings": true}
		}

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

		stateWithoutLive, err := nullAttributes(readResp.State.Raw, computedByRead)

		if err != nil {
			test.Fatal(err)
		}

		if !stateWithoutLive.Equal(want) {
			test.Errorf("%s: refreshed state = %v, want %v", name, readResp.State.Raw, want)
		}
	}
}

func TestResourceReadRefreshesTheEntry(test *testing.T) {
	var ctx context.Context = context.Background()
	var state CaptureSpec[testConfig, testResource] = testCaptureSpec
	var refreshed CaptureSpec[testConfig, testResource] = testCaptureSpec
	var backend *testBackend = &testBackend{live: liveCapture(map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"})}

	entry := newTestCaptureEntry(backend)

	backend.live["endpoint"] = map[string]any{"connector": map[string]any{"image": "ghcr.io/estuary/source-test:v2", "config": map[string]any{"host": "replica.example.com", "port": 5433, "password_sops": "ENC[AES256_GCM,data:abc,type:str]"}}}

	state.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true}
	state.Endpoint.Connector.Config.Port = 5432
	state.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}

	refreshed.Endpoint.Connector.Image = "ghcr.io/estuary/source-test:v2"
	refreshed.Endpoint.Connector.Config.Host = "replica.example.com"
	refreshed.Endpoint.Connector.Config.Port = 5433
	refreshed.Bindings = state.Bindings

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

	if readResp.Diagnostics.HasError() {
		test.Fatalf("read: %v", readResp.Diagnostics)
	}

	stateWithoutLive, err := nullAttributes(readResp.State.Raw, computedByRead)

	if err != nil {
		test.Fatal(err)
	}

	if !stateWithoutLive.Equal(want) {
		test.Errorf("refreshed state = %v, want the live image, host, port and no auto discover %v", readResp.State.Raw, want)
	}
}

func TestResourceReadKeepsZeroValues(test *testing.T) {
	var spec CaptureSpec[testConfig, testResource] = testCaptureSpec

	entry := newTestCaptureEntry(&testBackend{live: liveCapture(map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"})})

	spec.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}

	stateValue, err := entry.codec.Value("movies", spec)

	if err == nil {
		stateValue, err = withValue(stateValue, tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(0).WithAttributeName("disable"), false)
	}

	if err != nil {
		test.Fatal(err)
	}

	var readResp resource.ReadResponse = resource.ReadResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}}

	entry.Read(context.Background(), resource.ReadRequest{State: readResp.State}, &readResp)

	stateWithoutLive, err := nullAttributes(readResp.State.Raw, computedByRead)

	if err != nil {
		test.Fatal(err)
	}

	if readResp.Diagnostics.HasError() || !stateWithoutLive.Equal(stateValue) {
		test.Errorf("refreshed state = %v (%v), want disable = false kept as the live spec omits it", readResp.State.Raw, readResp.Diagnostics)
	}
}

func TestResourceReadLiveEntry(test *testing.T) {
	var ctx context.Context = context.Background()

	expectations := map[string]struct {
		live    map[string]any
		removed bool
		fail    bool
	}{
		"removes a deleted entry":         {live: nil, removed: true},
		"fails on an undecodable spec":    {live: map[string]any{"bindings": "not a list"}, fail: true},
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

		if failed := readResp.Diagnostics.HasError(); failed != expected.fail {
			test.Errorf("%s: failed = %v, want %v: %v", name, failed, expected.fail, readResp.Diagnostics)
			continue
		}

		if removed := readResp.State.Raw.IsNull(); removed != expected.removed {
			test.Errorf("%s: removed = %v, want %v", name, removed, expected.removed)
		}

		if readResp.Diagnostics.WarningsCount() > 0 {
			test.Errorf("%s: warnings = %v, want none", name, readResp.Diagnostics.Warnings())
		}

		stateWithoutLive, err := nullAttributes(readResp.State.Raw, computedByRead)

		if err != nil {
			test.Fatal(err)
		}

		if !expected.removed && !stateWithoutLive.Equal(stateValue) {
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
		map[string]any{"resource": map[string]any{"table": "events"}, "target": "acmeCo/events"},
	)}

	entry := newTestCaptureEntry(backend)

	prior.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true}
	prior.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "logs"}, Target: "acmeCo/logs"}}

	planned.AutoDiscover = prior.AutoDiscover
	planned.Bindings = []capture.Binding[testResource]{
		{Resource: testResource{Table: "logs", Priority: 1}, Target: "acmeCo/logs"},
		{Resource: testResource{Table: "users"}, Target: "acmeCo/users"},
		{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"},
		{Resource: testResource{Table: "events"}, Target: "acmeCo/events"},
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

	if len(warnings) != 1 || warnings[0].Summary() != "Existing bindings taken over in acmeCo/movies/source-test" {
		test.Fatalf("warnings = %v, want a single warning listing the taken over bindings", warnings)
	}

	if warning, isAttribute := warnings[0].(interface{ Path() path.Path }); !isAttribute || !warning.Path().Equal(path.Root("bindings")) {
		test.Errorf("warning = %v, want it at bindings", warnings[0])
	}

	for _, entry := range []string{"- acmeCo/users (bindings[1]): no field changes", "- acmeCo/events (bindings[3]): no field changes"} {
		if !strings.Contains(warnings[0].Detail(), entry) {
			test.Errorf("warning detail = %q, want it to list %q", warnings[0].Detail(), entry)
		}
	}

	if strings.Contains(warnings[0].Detail(), "acmeCo/logs") || strings.Contains(warnings[0].Detail(), "acmeCo/orders") {
		test.Errorf("warning detail = %q, want only the bindings already live and not managed", warnings[0].Detail())
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
		"warns when discovery adds new bindings":     {&capture.AutoDiscover{AddNewBindings: true}, []capture.Binding[testResource]{logs}, 1},
		"keeps a disabled binding without warning":   {&capture.AutoDiscover{AddNewBindings: true}, []capture.Binding[testResource]{logs, {Resource: users.Resource, Target: users.Target, Disable: true}}, 0},
		"skips discovery not adding new bindings":    {&capture.AutoDiscover{}, []capture.Binding[testResource]{logs}, 0},
		"skips captures without auto discover":       {nil, []capture.Binding[testResource]{logs}, 0},
		"lists every removed binding in one warning": {&capture.AutoDiscover{AddNewBindings: true}, []capture.Binding[testResource]{}, 1},
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
			if !strings.Contains(warning.Detail(), "disable = true") || !strings.Contains(warning.Detail(), "- acmeCo/users") {
				test.Errorf("%s: warning detail = %q, want the removed users binding listed with the disable advice", name, warning.Detail())
			}
		}

		if expected.planned != nil && len(expected.planned) == 0 && len(warnings) == 1 && !strings.Contains(warnings[0].Detail(), "- acmeCo/logs") {
			test.Errorf("%s: warning detail = %q, want the removed logs binding listed too", name, warnings[0].Detail())
		}
	}
}

func TestCodecBindingChanges(test *testing.T) {
	var path *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(0)
	var users capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "users"}, Target: "acmeCo/users"}

	codec, err := newCodec[captureDocument[testConfig, testResource]](testNamedConnector, captureType[testResource]())

	if err != nil {
		test.Fatal(err)
	}

	expectations := map[string]struct {
		planned capture.Binding[testResource]
		live    capture.Binding[testResource]
		want    []string
	}{
		"same binding":            {users, users, nil},
		"keeps the live backfill": {users, capture.Binding[testResource]{Resource: users.Resource, Target: users.Target, Backfill: 3}, nil},
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

func TestResourceModifyPlanWarnsAboutDiscoveredBindings(test *testing.T) {
	var ctx context.Context = context.Background()

	expectations := map[string]struct {
		autoDiscover *capture.AutoDiscover
		prior        *capture.AutoDiscover
		created      bool
		want         int
	}{
		"warns on creation adding new bindings":         {&capture.AutoDiscover{AddNewBindings: true}, nil, true, 1},
		"skips creation not adding new bindings":        {&capture.AutoDiscover{}, nil, true, 0},
		"skips creation without auto discover":          {nil, nil, true, 0},
		"warns on updates starting to add new bindings": {&capture.AutoDiscover{AddNewBindings: true}, &capture.AutoDiscover{}, false, 1},
		"skips updates already adding new bindings":     {&capture.AutoDiscover{AddNewBindings: true}, &capture.AutoDiscover{AddNewBindings: true}, false, 0},
	}

	for name, expected := range expectations {
		var spec CaptureSpec[testConfig, testResource] = testCaptureSpec

		entry := newTestCaptureEntry(&testBackend{live: liveCapture(map[string]any{"resource": map[string]any{"table": "users"}, "target": "acmeCo/users"})})

		spec.AutoDiscover = expected.autoDiscover
		spec.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "users"}, Target: "acmeCo/users"}}

		planValue, err := entry.codec.Value("movies", spec)

		if err != nil {
			test.Fatal(err)
		}

		var stateValue tftypes.Value = tftypes.NewValue(entry.codec.Schema().Type().TerraformType(ctx), nil)

		if !expected.created {
			spec.AutoDiscover = expected.prior

			if stateValue, err = entry.codec.Value("movies", spec); err != nil {
				test.Fatal(err)
			}
		}

		var modifyPlanReq resource.ModifyPlanRequest = resource.ModifyPlanRequest{
			State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue},
			Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
			Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: planValue},
		}
		var modifyPlanResp resource.ModifyPlanResponse = resource.ModifyPlanResponse{Plan: modifyPlanReq.Plan}

		entry.ModifyPlan(ctx, modifyPlanReq, &modifyPlanResp)

		var found int

		for _, warning := range modifyPlanResp.Diagnostics.Warnings() {
			if warning.Summary() == "Auto discovery adds unmanaged bindings in acmeCo/movies/source-test" && strings.Contains(warning.Detail(), "To manage one, declare it in bindings") {
				found++
			}
		}

		if modifyPlanResp.Diagnostics.HasError() || found != expected.want {
			test.Errorf("%s: diagnostics = %v, want %d discovered bindings warnings", name, modifyPlanResp.Diagnostics, expected.want)
		}
	}
}

func TestResourceReadRefreshesSetShardSettings(test *testing.T) {
	var ctx context.Context = context.Background()

	expectations := map[string]struct {
		state catalog.Shards
		want  catalog.Shards
	}{
		"refreshes every live setting":         {catalog.Shards{"logLevel": "debug"}, catalog.Shards{"logLevel": "info", "disable": true, "runtimeV2": true}},
		"keeps a zero setting live leaves out": {catalog.Shards{"maxTxnDuration": ""}, catalog.Shards{"logLevel": "info", "disable": true, "runtimeV2": true, "maxTxnDuration": ""}},
		"refreshes unset shards too":           {nil, catalog.Shards{"logLevel": "info", "disable": true, "runtimeV2": true}},
	}

	for name, expected := range expectations {
		var state CaptureSpec[testConfig, testResource] = testCaptureSpec
		var refreshed CaptureSpec[testConfig, testResource] = testCaptureSpec
		var backend *testBackend = &testBackend{live: liveCapture()}

		entry := newTestCaptureEntry(backend)

		backend.live["shards"] = map[string]any{"logLevel": "info", "disable": true, "runtimeV2": true}

		state.Shards = expected.state
		refreshed.Shards = expected.want

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

		if readResp.Diagnostics.HasError() {
			test.Fatalf("%s: read: %v", name, readResp.Diagnostics)
		}

		stateWithoutLive, err := nullAttributes(readResp.State.Raw, computedByRead)

		if err != nil {
			test.Fatal(err)
		}

		if !stateWithoutLive.Equal(want) {
			test.Errorf("%s: refreshed state = %v, want %v", name, readResp.State.Raw, want)
		}
	}
}

func TestResourceModifyPlanWarnsAboutDisabledLiveEntries(test *testing.T) {
	expectations := map[string]struct {
		live   catalog.Shards
		shards catalog.Shards
		warn   bool
	}{
		"warns about a live entry disabled outside Terraform": {catalog.Shards{"disable": true}, nil, true},
		"warns when the shards set leave disable out":         {catalog.Shards{"disable": true}, catalog.Shards{"logLevel": "debug"}, true},
		"leaves a disable set in the configuration":           {catalog.Shards{"disable": true}, catalog.Shards{"disable": false}, false},
		"leaves an enabled live entry":                        {catalog.Shards{"disable": false}, nil, false},
		"leaves a live entry without shards":                  {nil, nil, false},
	}

	entry := newTestCaptureEntry(&testBackend{})

	for name, expected := range expectations {
		var configured CaptureSpec[testConfig, testResource] = testCaptureSpec
		var live CaptureSpec[testConfig, testResource] = testCaptureSpec

		configured.Shards = expected.shards
		live.Shards = expected.live

		config, err := entry.codec.Value("movies", configured)

		if err != nil {
			test.Fatal(err)
		}

		state, err := entry.codec.Value("movies", live)

		if err != nil {
			test.Fatal(err)
		}

		warnings := entry.warnDisabledLive(config, state)

		if warned := len(warnings) == 1 && warnings[0].Summary() == "Disabled outside Terraform in acmeCo/movies/source-test"; warned != expected.warn || len(warnings) > 1 {
			test.Errorf("%s: warnings = %v, want a disabled warning %v", name, warnings, expected.warn)
		}
	}
}

func TestPlanShards(test *testing.T) {
	var live catalog.Shards = catalog.Shards{"disable": true, "flags": map[string]any{"enable-runtime-v2": "true"}}

	expectations := map[string]struct {
		config  catalog.Shards
		created bool
		want    catalog.Shards
	}{
		"plans the configured shards":                  {catalog.Shards{"disable": true}, false, catalog.Shards{"disable": true}},
		"keeps the live shards when unset":             {nil, false, live},
		"plans no shards for an entry not created yet": {nil, true, nil},
	}

	entry := newTestCaptureEntry(&testBackend{})

	for name, expected := range expectations {
		var configured CaptureSpec[testConfig, testResource] = testCaptureSpec
		var stored CaptureSpec[testConfig, testResource] = testCaptureSpec
		var planned CaptureSpec[testConfig, testResource] = testCaptureSpec

		configured.Shards = expected.config
		stored.Shards = live
		planned.Shards = expected.want

		config, err := entry.codec.Value("movies", configured)

		if err != nil {
			test.Fatal(err)
		}

		state, err := entry.codec.Value("movies", stored)

		if err != nil {
			test.Fatal(err)
		}

		if expected.created {
			state = tftypes.NewValue(entry.codec.valueType, nil)
		}

		want, err := entry.codec.Value("movies", planned)

		if err != nil {
			test.Fatal(err)
		}

		plan, err := planShards(config, config, state)

		if err != nil {
			test.Fatal(err)
		}

		shards, _ := attributeValue(plan, tftypes.NewAttributePath().WithAttributeName("shards"))
		wanted, _ := attributeValue(want, tftypes.NewAttributePath().WithAttributeName("shards"))

		if !shards.Equal(wanted) && !(shards.IsNull() && wanted.IsNull()) {
			test.Errorf("%s: planned shards = %v, want %v", name, shards, wanted)
		}
	}
}

func TestResourceReadRefreshesLiveOnlyFields(test *testing.T) {
	var ctx context.Context = context.Background()
	var state CaptureSpec[testConfig, testResource] = testCaptureSpec
	var refreshed CaptureSpec[testConfig, testResource] = testCaptureSpec
	var backend *testBackend = &testBackend{live: liveCapture()}

	entry := newTestCaptureEntry(backend)

	backend.live["autoDiscover"] = map[string]any{"addNewBindings": true}
	backend.live["endpoint"] = map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{
		"host": "db.example.com", "ratio": 0.5, "readOnly": false, "tunnel": map[string]any{"sshEndpoint": "ssh://bastion:22"}, "password_sops": "ENC[AES256_GCM,data:abc,type:str]",
	}}}

	refreshed.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true}
	refreshed.Endpoint.Connector.Config.Ratio = 0.5
	refreshed.Endpoint.Connector.Config.Tunnel = &testTunnel{SSHEndpoint: "ssh://bastion:22"}

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

	if readResp.Diagnostics.HasError() {
		test.Fatalf("read: %v", readResp.Diagnostics)
	}

	stateWithoutLive, err := nullAttributes(readResp.State.Raw, computedByRead)

	if err != nil {
		test.Fatal(err)
	}

	if !stateWithoutLive.Equal(want) {
		test.Errorf("refreshed state = %v, want the live only fields, without the empty ones nor the secrets", readResp.State.Raw)
	}
}

func TestCodecLiveValueNullsUnencryptedSecrets(test *testing.T) {
	var sealed map[string]any = liveCapture()

	codec, err := newCodec[captureDocument[testConfig, testResource]](testNamedConnector, captureType[testResource]())

	if err != nil {
		test.Fatal(err)
	}

	sealed["endpoint"] = map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com", "password": "hunter2"}}}

	live, _, err := codec.liveValue("movies", sealed)

	if err != nil {
		test.Fatal(err)
	}

	password, err := attributeValue(live, tftypes.NewAttributePath().WithAttributeName("config").WithAttributeName("password"))

	if err != nil || !password.IsNull() {
		test.Errorf("live password = %v (%v), want null so refreshes never store it", password, err)
	}

	if host, _ := attributeValue(live, tftypes.NewAttributePath().WithAttributeName("config").WithAttributeName("host")); host.IsNull() {
		test.Error("live host is null, want only secrets nulled")
	}
}

func TestCodecLiveValueRedactsCredentials(test *testing.T) {
	type credentialsConfig struct {
		Address     string `yaml:"address" credentials:"forbidden"`
		SSHEndpoint string `yaml:"sshEndpoint" credentials:"password"`
		Password    string `yaml:"password,omitempty"`
	}

	var sealed map[string]any = liveCapture()
	var addressPath *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName("config").WithAttributeName("address")
	var endpointPath *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName("config").WithAttributeName("ssh_endpoint")

	codec, err := newCodec[captureDocument[credentialsConfig, testResource]](testNamedConnector, captureType[testResource]())

	if err != nil {
		test.Fatal(err)
	}

	sealed["endpoint"] = map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"address": "mongodb://flow_capture:secret@123.4.567.89:27017", "sshEndpoint": "ssh://ec2-user:secret@123.4.567.89:22"}}}

	live, redactedPaths, err := codec.liveValue("movies", sealed)

	if err != nil {
		test.Fatal(err)
	}

	address, _ := attributeValue(live, addressPath)

	if !address.Equal(tftypes.NewValue(tftypes.String, "mongodb://***@123.4.567.89:27017")) {
		test.Errorf("live address = %v, want the credentials redacted", address)
	}

	endpoint, _ := attributeValue(live, endpointPath)

	if !endpoint.Equal(tftypes.NewValue(tftypes.String, "ssh://ec2-user:***@123.4.567.89:22")) {
		test.Errorf("live ssh_endpoint = %v, want only the password redacted", endpoint)
	}

	if len(redactedPaths) != 2 {
		test.Errorf("redacted paths = %v, want config.address and config.ssh_endpoint", redactedPaths)
	}
}
