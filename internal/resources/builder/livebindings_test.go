package builder

import (
	"context"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestResourcesHaveNoLiveBindings(test *testing.T) {
	entry := newTestCaptureEntry(&testBackend{})

	codec, err := newCodec[materializationDocument[testConfig, testResource], MaterializationSpec[testConfig, testResource]](testNamedConnector, materializationType[testConfig, testResource]())

	if err != nil {
		test.Fatal(err)
	}

	if _, found := entry.codec.Schema().Attributes[liveBindingsAttribute]; found {
		test.Error("capture has live_bindings, want it only on data sources")
	}

	if _, found := codec.Schema().Attributes[liveBindingsAttribute]; found {
		test.Error("materialization has live_bindings, want it only on data sources")
	}
}

func TestResourceImportStateLeavesDiscoveredBindingsUnmanaged(test *testing.T) {
	var ctx context.Context = context.Background()
	var backend *testBackend = &testBackend{live: liveCapture(map[string]any{"resource": map[string]any{"table": "events"}, "target": "acmeCo/events"})}
	var managed []tftypes.Value

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

	if len(managed) != 0 {
		test.Errorf("bindings = %v, want the discovered events binding left out of state", managed)
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
