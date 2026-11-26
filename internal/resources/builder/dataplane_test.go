package builder

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Data planes of the data plane tests: the one a resource configures, the provider one, and the one a task runs in.
const (
	ownDataPlane      string = "ops/dp/private/acmeCo/own"
	providerDataPlane string = "ops/dp/public/provider"
	liveDataPlane     string = "ops/dp/public/live"
)

// newDataPlaneEntry returns a capture resource using backend under the acmeCo/ tenant and the provider data plane.
func newDataPlaneEntry(backend *testBackend, providerDataPlane string) *testCaptureEntry {
	var configureResp resource.ConfigureResponse

	entry := NewCapture[testConfig, testResource](testNamedConnector)().(*testCaptureEntry)

	entry.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/", DataPlane: providerDataPlane},
	}, &configureResp)

	return entry
}

// dataPlaneState returns the state of the capture name running in dataPlane, nil when state holds none.
func dataPlaneState(test *testing.T, entry *testCaptureEntry, name string, dataPlane any) tftypes.Value {
	test.Helper()

	state, err := entry.codec.Value(name, testCaptureSpec)

	if err == nil {
		state, err = withCatalogName(state, entry.catalogName(name))
	}

	if err == nil {
		state, err = withValue(state, dataPlanePath, dataPlane)
	}

	if err != nil {
		test.Fatal(err)
	}

	return state
}

// planDataPlaneOf plans the capture name over state with data_plane configured, nil when the configuration omits it,
// which Terraform then proposes as unknown. Untargeted adds a binding whose target the plan resolves by discovery.
func planDataPlaneOf(test *testing.T, entry *testCaptureEntry, state tftypes.Value, name string, configured any, untargeted bool) resource.ModifyPlanResponse {
	test.Helper()

	var ctx context.Context = context.Background()
	var spec CaptureSpec[testConfig, testResource] = testCaptureSpec
	var bindingName *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName(bindingsAttribute).WithElementKeyInt(0).WithAttributeName(bindingNameAttribute)

	if untargeted {
		spec.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "users"}}}
	}

	configValue, err := entry.codec.Value(name, spec)

	if err == nil {
		configValue, err = withValue(configValue, dataPlanePath, configured)
	}

	if untargeted && err == nil {
		configValue, err = withValue(configValue, bindingTargetPath(0), nil)
	}

	if untargeted && err == nil {
		configValue, err = withValue(configValue, bindingName, nil)
	}

	var planValue tftypes.Value = configValue

	if configured == nil && err == nil {
		planValue, err = withValue(planValue, dataPlanePath, tftypes.UnknownValue)
	}

	if untargeted && err == nil {
		planValue, err = withValue(planValue, bindingTargetPath(0), tftypes.UnknownValue)
	}

	if untargeted && err == nil {
		planValue, err = withValue(planValue, bindingName, tftypes.UnknownValue)
	}

	if err != nil {
		test.Fatal(err)
	}

	var modifyPlanReq resource.ModifyPlanRequest = resource.ModifyPlanRequest{
		State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: state},
		Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
		Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: configValue},
	}
	var modifyPlanResp resource.ModifyPlanResponse = resource.ModifyPlanResponse{Plan: modifyPlanReq.Plan}

	entry.ModifyPlan(ctx, modifyPlanReq, &modifyPlanResp)

	return modifyPlanResp
}

// noState returns the state of an entry not created yet.
func noState(entry *testCaptureEntry) tftypes.Value {
	return tftypes.NewValue(entry.codec.Schema().Type().TerraformType(context.Background()), nil)
}

func TestResourceDataPlaneOfANewTaskPrefersTheResourceOverTheProvider(test *testing.T) {
	var ctx context.Context = context.Background()

	expectations := map[string]struct {
		configured any
		provider   string
		planned    any
		published  string
		stored     string
	}{
		"the resource data plane wins":           {ownDataPlane, providerDataPlane, ownDataPlane, ownDataPlane, ownDataPlane},
		"the resource data plane needs no other": {ownDataPlane, "", ownDataPlane, ownDataPlane, ownDataPlane},
		"the provider data plane is the default": {nil, providerDataPlane, providerDataPlane, providerDataPlane, providerDataPlane},
		"Estuary picks without either":           {nil, "", tftypes.UnknownValue, "", liveDataPlane},
	}

	for name, expected := range expectations {
		var backend *testBackend = &testBackend{live: liveCapture(), dataPlane: liveDataPlane}

		entry := newDataPlaneEntry(backend, expected.provider)

		modifyPlanResp := planDataPlaneOf(test, entry, noState(entry), "movies", expected.configured, false)

		if modifyPlanResp.Diagnostics.HasError() {
			test.Errorf("%s: %v", name, modifyPlanResp.Diagnostics)
			continue
		}

		if got := blockAttribute(modifyPlanResp.Plan.Raw, dataPlaneAttribute, tftypes.String); !got.Equal(tftypes.NewValue(tftypes.String, expected.planned)) {
			test.Errorf("%s: planned data_plane = %v, want %v", name, got, expected.planned)
		}

		var createResp resource.CreateResponse = resource.CreateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

		entry.Create(ctx, resource.CreateRequest{Plan: modifyPlanResp.Plan}, &createResp)

		if createResp.Diagnostics.HasError() {
			test.Errorf("%s: create: %v", name, createResp.Diagnostics)
			continue
		}

		if !reflect.DeepEqual(backend.dataPlanes, []string{expected.published}) {
			test.Errorf("%s: published on %q, want only %q", name, backend.dataPlanes, expected.published)
		}

		if got := blockAttribute(createResp.State.Raw, dataPlaneAttribute, tftypes.String); !got.Equal(tftypes.NewValue(tftypes.String, expected.stored)) {
			test.Errorf("%s: stored data_plane = %v, want %s", name, got, expected.stored)
		}
	}
}

func TestResourceDataPlaneCreateStoresNoneWhenEstuaryNamesNone(test *testing.T) {
	var backend *testBackend = &testBackend{live: liveCapture(), dataPlaneErr: errors.New("control plane unreachable")}

	entry := newDataPlaneEntry(backend, "")

	modifyPlanResp := planDataPlaneOf(test, entry, noState(entry), "movies", nil, false)

	var createResp resource.CreateResponse = resource.CreateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

	entry.Create(context.Background(), resource.CreateRequest{Plan: modifyPlanResp.Plan}, &createResp)

	if modifyPlanResp.Diagnostics.HasError() || createResp.Diagnostics.HasError() {
		test.Fatalf("plan: %v, create: %v", modifyPlanResp.Diagnostics, createResp.Diagnostics)
	}

	if got := blockAttribute(createResp.State.Raw, dataPlaneAttribute, tftypes.String); !got.IsKnown() || !got.IsNull() {
		test.Errorf("stored data_plane = %v, want null for the next refresh to fill in, never unknown", got)
	}
}

func TestResourceDataPlaneImportFillsState(test *testing.T) {
	var ctx context.Context = context.Background()
	var backend *testBackend = &testBackend{live: liveCapture(), dataPlane: liveDataPlane}
	var want tftypes.Value = tftypes.NewValue(tftypes.String, liveDataPlane)

	entry := newDataPlaneEntry(backend, providerDataPlane)

	var importResp resource.ImportStateResponse = resource.ImportStateResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: noState(entry)}}

	entry.ImportState(ctx, resource.ImportStateRequest{ID: "acmeCo/movies/source-test"}, &importResp)

	if importResp.Diagnostics.HasError() {
		test.Fatalf("import: %v", importResp.Diagnostics)
	}

	if got := blockAttribute(importResp.State.Raw, dataPlaneAttribute, tftypes.String); !got.Equal(want) {
		test.Fatalf("imported data_plane = %v, want the live one, %v", got, want)
	}

	for name, configured := range map[string]any{"without data_plane": nil, "with the live data_plane": liveDataPlane} {
		modifyPlanResp := planDataPlaneOf(test, entry, importResp.State.Raw, "acmeCo/movies", configured, false)

		if modifyPlanResp.Diagnostics.HasError() || len(modifyPlanResp.RequiresReplace) > 0 {
			test.Errorf("%s: diagnostics = %v, replaces %v, want a plan without changes", name, modifyPlanResp.Diagnostics, modifyPlanResp.RequiresReplace)
		}

		if got := blockAttribute(modifyPlanResp.Plan.Raw, dataPlaneAttribute, tftypes.String); !got.Equal(want) {
			test.Errorf("%s: planned data_plane = %v, want the imported one, %v", name, got, want)
		}
	}
}

func TestResourceDataPlaneRefreshReadsTheLivePlane(test *testing.T) {
	var ctx context.Context = context.Background()

	expectations := map[string]struct {
		state   any
		live    string
		readErr error
		want    any
	}{
		"fills state written before data_plane existed": {nil, liveDataPlane, nil, liveDataPlane},
		"shows a task migrated to another data plane":   {ownDataPlane, liveDataPlane, nil, liveDataPlane},
		"keeps the state one when the read fails":       {ownDataPlane, "", errors.New("control plane unreachable"), ownDataPlane},
		"keeps the state one when Estuary names none":   {ownDataPlane, "", nil, ownDataPlane},
		"stays empty when Estuary names none":           {nil, "", nil, nil},
	}

	for name, expected := range expectations {
		entry := newDataPlaneEntry(&testBackend{live: liveCapture(), dataPlane: expected.live, dataPlaneErr: expected.readErr}, providerDataPlane)

		var state tfsdk.State = tfsdk.State{Schema: entry.codec.Schema(), Raw: dataPlaneState(test, entry, "movies", expected.state)}
		var readResp resource.ReadResponse = resource.ReadResponse{State: state}

		entry.Read(ctx, resource.ReadRequest{State: state}, &readResp)

		if readResp.Diagnostics.HasError() || readResp.Diagnostics.WarningsCount() > 0 {
			test.Errorf("%s: diagnostics = %v, want a read that never fails or warns over the data plane", name, readResp.Diagnostics)
			continue
		}

		if got := blockAttribute(readResp.State.Raw, dataPlaneAttribute, tftypes.String); !got.Equal(tftypes.NewValue(tftypes.String, expected.want)) {
			test.Errorf("%s: refreshed data_plane = %v, want %v", name, got, expected.want)
		}
	}
}

func TestResourceDataPlaneChangeFailsThePlan(test *testing.T) {
	var backend *testBackend = &testBackend{live: liveCapture(), discovered: map[string]map[string]any{"acmeCo/movies/users": {"table": "users"}}}

	entry := newDataPlaneEntry(backend, providerDataPlane)

	modifyPlanResp := planDataPlaneOf(test, entry, dataPlaneState(test, entry, "movies", liveDataPlane), "movies", ownDataPlane, true)

	var failures = modifyPlanResp.Diagnostics.Errors()

	if len(failures) != 1 {
		test.Fatalf("diagnostics = %v, want one error", modifyPlanResp.Diagnostics)
	}

	if located, isLocated := failures[0].(interface{ Path() path.Path }); !isLocated || !located.Path().Equal(path.Root(dataPlaneAttribute)) {
		test.Errorf("error = %v, want it on the data_plane attribute", failures[0])
	}

	for _, want := range []string{liveDataPlane, ownDataPlane, "can't move a live capture", "Estuary support", "once it's migrated set data_plane"} {
		if !strings.Contains(failures[0].Detail(), want) {
			test.Errorf("error = %q, want it to say %q", failures[0].Detail(), want)
		}
	}

	if len(modifyPlanResp.RequiresReplace) > 0 || len(backend.discovers) > 0 || backend.published != nil {
		test.Errorf("replaces %v, discovered %d times, published %v, want nothing replaced, discovered or published",
			modifyPlanResp.RequiresReplace, len(backend.discovers), backend.published)
	}
}

func TestResourceDataPlaneIsAcceptedWhileStateHoldsNone(test *testing.T) {
	entry := newDataPlaneEntry(&testBackend{live: liveCapture()}, providerDataPlane)

	modifyPlanResp := planDataPlaneOf(test, entry, dataPlaneState(test, entry, "movies", nil), "movies", ownDataPlane, false)

	if modifyPlanResp.Diagnostics.HasError() || len(modifyPlanResp.RequiresReplace) > 0 {
		test.Fatalf("diagnostics = %v, replaces %v, want the configured data plane accepted", modifyPlanResp.Diagnostics, modifyPlanResp.RequiresReplace)
	}

	if got := blockAttribute(modifyPlanResp.Plan.Raw, dataPlaneAttribute, tftypes.String); !got.Equal(tftypes.NewValue(tftypes.String, ownDataPlane)) {
		test.Errorf("planned data_plane = %v, want the configured one", got)
	}
}

func TestResourceDataPlaneProviderChangeKeepsExistingResources(test *testing.T) {
	var ctx context.Context = context.Background()

	for name, stored := range map[string]any{"a task with its data plane in state": liveDataPlane, "a task without one in state": nil} {
		var backend *testBackend = &testBackend{live: liveCapture()}

		entry := newDataPlaneEntry(backend, providerDataPlane)

		var state tftypes.Value = dataPlaneState(test, entry, "movies", stored)

		modifyPlanResp := planDataPlaneOf(test, entry, state, "movies", nil, false)

		if modifyPlanResp.Diagnostics.HasError() || len(modifyPlanResp.RequiresReplace) > 0 {
			test.Errorf("%s: diagnostics = %v, replaces %v, want the provider data plane to change nothing", name, modifyPlanResp.Diagnostics, modifyPlanResp.RequiresReplace)
			continue
		}

		if !modifyPlanResp.Plan.Raw.Equal(state) {
			test.Errorf("%s: plan = %v, want the state, %v, so the plan shows no changes", name, modifyPlanResp.Plan.Raw, state)
		}

		var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

		entry.Update(ctx, resource.UpdateRequest{Plan: modifyPlanResp.Plan, State: tfsdk.State{Schema: entry.codec.Schema(), Raw: state}}, &updateResp)

		if updateResp.Diagnostics.HasError() {
			test.Errorf("%s: update: %v", name, updateResp.Diagnostics)
		}

		for _, dataPlane := range backend.dataPlanes {
			if dataPlane == providerDataPlane {
				test.Errorf("%s: published on the provider data plane, want only its own: %q", name, backend.dataPlanes)
			}
		}
	}
}

func TestResourceDataPlaneReplacementPlansANewTask(test *testing.T) {
	expectations := map[string]struct {
		configured any
		provider   string
		want       any
	}{
		"the resource data plane":      {ownDataPlane, providerDataPlane, ownDataPlane},
		"the provider data plane":      {nil, providerDataPlane, providerDataPlane},
		"the one Estuary picks":        {nil, "", tftypes.UnknownValue},
		"the one the old task runs in": {liveDataPlane, "", liveDataPlane},
	}

	for name, expected := range expectations {
		entry := newDataPlaneEntry(&testBackend{live: liveCapture()}, expected.provider)

		modifyPlanResp := planDataPlaneOf(test, entry, dataPlaneState(test, entry, "movies", liveDataPlane), "films", expected.configured, false)

		if modifyPlanResp.Diagnostics.HasError() {
			test.Errorf("%s: %v, want no error for the new task of a renamed entry", name, modifyPlanResp.Diagnostics)
			continue
		}

		if len(modifyPlanResp.RequiresReplace) == 0 {
			test.Errorf("%s: nothing requires replace, want the renamed entry replaced", name)
		}

		if got := blockAttribute(modifyPlanResp.Plan.Raw, dataPlaneAttribute, tftypes.String); !got.Equal(tftypes.NewValue(tftypes.String, expected.want)) {
			test.Errorf("%s: planned data_plane = %v, want %v, ignoring the state one", name, got, expected.want)
		}
	}
}

func TestResourceDataPlaneOfPlanTimeDiscovery(test *testing.T) {
	expectations := map[string]struct {
		created    bool
		stored     any
		configured any
		want       string
	}{
		"a new task on the resource data plane":         {true, nil, ownDataPlane, ownDataPlane},
		"a new task on the provider data plane":         {true, nil, nil, providerDataPlane},
		"a live task on its own data plane":             {false, liveDataPlane, nil, liveDataPlane},
		"a live task on the configured one it runs in":  {false, liveDataPlane, liveDataPlane, liveDataPlane},
		"a live task state holds no data plane of":      {false, nil, nil, ""},
		"a live task on the configured one without one": {false, nil, ownDataPlane, ownDataPlane},
	}

	for name, expected := range expectations {
		var backend *testBackend = &testBackend{discovered: map[string]map[string]any{"acmeCo/movies/users": {"table": "users"}}}

		entry := newDataPlaneEntry(backend, providerDataPlane)

		var state tftypes.Value = noState(entry)

		if !expected.created {
			state = dataPlaneState(test, entry, "movies", expected.stored)
		}

		modifyPlanResp := planDataPlaneOf(test, entry, state, "movies", expected.configured, true)

		if modifyPlanResp.Diagnostics.HasError() {
			test.Errorf("%s: %v", name, modifyPlanResp.Diagnostics)
			continue
		}

		if !reflect.DeepEqual(backend.dataPlanes, []string{expected.want}) {
			test.Errorf("%s: discovered on %q, want once on %q", name, backend.dataPlanes, expected.want)
		}
	}
}

func TestResourceDataPlaneOfPublishTimeDiscovery(test *testing.T) {
	var backend *testBackend = &testBackend{live: liveCapture()}
	var spec CaptureSpec[testConfig, testResource] = testCaptureSpec

	spec.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "users"}, Target: "acmeCo/users"}}

	entry := newDataPlaneEntry(backend, providerDataPlane)

	plan, err := entry.codec.Value("movies", spec)

	if err == nil {
		plan, err = withValue(plan, dataPlanePath, ownDataPlane)
	}

	if err != nil {
		test.Fatal(err)
	}

	var createResp resource.CreateResponse = resource.CreateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

	entry.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: entry.codec.Schema(), Raw: plan}}, &createResp)

	if createResp.Diagnostics.HasError() {
		test.Fatalf("create: %v", createResp.Diagnostics)
	}

	if backend.discovery == nil || !reflect.DeepEqual(backend.dataPlanes, []string{ownDataPlane}) {
		test.Errorf("discovery = %v on %q, want the capture discovered and published on its data plane", backend.discovery, backend.dataPlanes)
	}
}

func TestResourceDataPlaneOfTheMaterializationDryRun(test *testing.T) {
	var ctx context.Context = context.Background()
	var configureResp resource.ConfigureResponse
	var backend *testBackend = &testBackend{
		live: map[string]any{
			"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com"}}},
			"bindings": []any{map[string]any{"resource": map[string]any{"table": "orders", "schema": "public"}, "source": "acmeCo/orders"}},
		},
	}

	entry := NewMaterialization[testWarehouseConfig, testWarehouseResource](testNamedConnector)().(*testWarehouseEntry)

	entry.Configure(ctx, resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/", DataPlane: providerDataPlane}}, &configureResp)

	spec := func(table string) MaterializationSpec[testWarehouseConfig, testWarehouseResource] {
		return MaterializationSpec[testWarehouseConfig, testWarehouseResource]{
			Endpoint: endpoint.ConnectorSpec[testWarehouseConfig]{Connector: endpoint.Connector[testWarehouseConfig]{Image: testNamedConnector.Image, Config: testWarehouseConfig{Host: "db.example.com"}}},
			Bindings: []materialization.Binding[testWarehouseResource]{{
				Resource: testWarehouseResource{Table: table, Schema: "public"},
				Source:   materialization.BindingSource{Name: "acmeCo/orders"},
			}},
		}
	}

	state, err := entry.codec.Value("warehouse", spec("orders"))

	if err == nil {
		state, err = withValue(state, dataPlanePath, liveDataPlane)
	}

	if err != nil {
		test.Fatal(err)
	}

	plan, err := entry.codec.Value("warehouse", spec("orders_v2"))

	if err == nil {
		plan, err = withValue(plan, dataPlanePath, tftypes.UnknownValue)
	}

	if err == nil {
		plan, err = entry.planCatalogName(plan)
	}

	if err == nil {
		plan, _, err = entry.planDataPlane(plan, tftypes.Value{}, state)
	}

	if err == nil {
		_, err = entry.planResourcePaths(ctx, plan, state)
	}

	if err != nil {
		test.Fatal(err)
	}

	if !reflect.DeepEqual(backend.dataPlanes, []string{liveDataPlane}) {
		test.Errorf("validated on %q, want once on the data plane the materialization runs in, not the provider one", backend.dataPlanes)
	}
}
