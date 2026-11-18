package builder

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func bindingTargetPath(index int) *tftypes.AttributePath {
	return tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(index).WithAttributeName("target")
}

func TestResourceModifyPlanPlansNoMaterializationTargets(test *testing.T) {
	var ctx context.Context = context.Background()
	var configureResp resource.ConfigureResponse
	var spec MaterializationSpec[testConfig, testResource] = MaterializationSpec[testConfig, testResource]{
		Endpoint: endpoint.ConnectorSpec[testConfig]{Connector: endpoint.Connector[testConfig]{Image: testNamedConnector.Image, Config: testConfig{Host: "db.example.com"}}},
		Bindings: []materialization.Binding[testResource]{{Resource: testResource{Table: "orders"}, Source: materialization.BindingSource{Name: "acmeCo/orders"}}},
	}

	entry := NewMaterialization[testConfig, testResource](testNamedConnector)().(*entryResource[materializationDocument[testConfig, testResource], MaterializationSpec[testConfig, testResource]])

	entry.Configure(ctx, resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: &testBackend{}, Tenant: "acmeCo/"}}, &configureResp)

	config, err := entry.codec.Value("warehouse", spec)

	if err != nil {
		test.Fatal(err)
	}

	expectations := map[string]tftypes.Value{
		"on create": tftypes.NewValue(entry.codec.valueType, nil),
		"on update": config,
	}

	for name, state := range expectations {
		var modifyPlanReq resource.ModifyPlanRequest = resource.ModifyPlanRequest{
			State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: state},
			Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: config},
			Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: config},
		}
		var modifyPlanResp resource.ModifyPlanResponse = resource.ModifyPlanResponse{Plan: modifyPlanReq.Plan}

		entry.ModifyPlan(ctx, modifyPlanReq, &modifyPlanResp)

		if modifyPlanResp.Diagnostics.HasError() {
			test.Errorf("%s: diagnostics = %v, want a materialization planned without targets", name, modifyPlanResp.Diagnostics)
		}
	}
}

func TestResourceModifyPlanPlansTargets(test *testing.T) {
	var ctx context.Context = context.Background()
	var users testResource = testResource{Table: "users"}
	var orders testResource = testResource{Table: "orders"}
	var discovered map[string]map[string]any = map[string]map[string]any{
		"acmeCo/movies/Public/Users":  {"table": "users"},
		"acmeCo/movies/Public/Orders": {"table": "orders", "extra": true},
	}

	expectations := map[string]struct {
		state     []capture.Binding[testResource]
		live      []any
		config    []capture.Binding[testResource]
		proposed  []any
		unknown   bool
		unplanned string
		want      []any
		discovers int
		wantError string
	}{
		"defaults to the discovered target": {
			config:    []capture.Binding[testResource]{{Resource: users}},
			proposed:  []any{tftypes.UnknownValue},
			want:      []any{"acmeCo/movies/Public/Users"},
			discovers: 1,
		},
		"fails on a resource discovery doesn't find": {
			config:    []capture.Binding[testResource]{{Resource: users}, {Resource: testResource{Table: "missing"}}},
			proposed:  []any{tftypes.UnknownValue, tftypes.UnknownValue},
			discovers: 1,
			wantError: "discovery found no resource for missing.",
		},
		"keeps the state target of the same resource": {
			state:     []capture.Binding[testResource]{{Resource: users, Target: "acmeCo/custom/users"}},
			config:    []capture.Binding[testResource]{{Resource: orders}, {Resource: users}},
			proposed:  []any{"acmeCo/custom/users", tftypes.UnknownValue},
			want:      []any{"acmeCo/movies/Public/Orders", "acmeCo/custom/users"},
			discovers: 1,
		},
		"takes the live target of an existing binding": {
			state:    []capture.Binding[testResource]{},
			live:     []any{map[string]any{"resource": map[string]any{"table": "users"}, "target": "acmeCo/legacy/users"}},
			config:   []capture.Binding[testResource]{{Resource: users}},
			proposed: []any{tftypes.UnknownValue},
			want:     []any{"acmeCo/legacy/users"},
		},
		"discovers with shards the configuration doesn't set": {
			config:    []capture.Binding[testResource]{{Resource: users}},
			proposed:  []any{tftypes.UnknownValue},
			unplanned: "shards",
			want:      []any{"acmeCo/movies/Public/Users"},
			discovers: 1,
		},
		"fails on a target the plan can't resolve from a known configuration": {
			config:    []capture.Binding[testResource]{{Resource: users}},
			proposed:  []any{tftypes.UnknownValue},
			unplanned: "image",
			wantError: "the target of some bindings couldn't be planned",
		},
		"keeps a configured target": {
			config:   []capture.Binding[testResource]{{Resource: users, Target: "acmeCo/mine"}},
			proposed: []any{"acmeCo/mine"},
			want:     []any{"acmeCo/mine"},
		},
		"stays unknown until the resources are known": {
			state:    []capture.Binding[testResource]{{Resource: users, Target: "acmeCo/custom/users"}},
			config:   []capture.Binding[testResource]{{Resource: orders}},
			proposed: []any{"acmeCo/custom/users"},
			unknown:  true,
			want:     []any{tftypes.UnknownValue},
		},
	}

	for name, expected := range expectations {
		var configSpec CaptureSpec[testConfig, testResource] = testCaptureSpec

		var backend *testBackend = &testBackend{live: liveCapture(expected.live...), discovered: discovered}

		entry := newTestCaptureEntry(backend)

		configSpec.Bindings = expected.config

		configValue, err := entry.codec.Value("movies", configSpec)

		if err != nil {
			test.Fatal(err)
		}

		for index, binding := range expected.config {
			if binding.Target == "" && err == nil {
				configValue, err = withValue(configValue, bindingTargetPath(index), nil)
			}

			if err == nil {
				configValue, err = withValue(configValue, tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(index).WithAttributeName("name"), nil)
			}
		}

		if expected.unknown && err == nil {
			configValue, err = withValue(configValue, tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(0).WithAttributeName("resource").WithAttributeName("table"), tftypes.UnknownValue)
		}

		var planValue tftypes.Value = configValue

		for index, target := range expected.proposed {
			if err == nil {
				planValue, err = withValue(planValue, bindingTargetPath(index), target)
			}

			if err == nil {
				planValue, err = withValue(planValue, tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(index).WithAttributeName("name"), tftypes.UnknownValue)
			}
		}

		if expected.unplanned == "shards" && err == nil {
			configValue, err = withValue(configValue, shardsPath, nil)

			if err == nil {
				planValue, err = withValue(planValue, shardsPath, tftypes.UnknownValue)
			}
		}

		if expected.unplanned == "image" && err == nil {
			planValue, err = withValue(planValue, tftypes.NewAttributePath().WithAttributeName("image"), tftypes.UnknownValue)
		}

		if err != nil {
			test.Fatal(err)
		}

		var stateValue tftypes.Value = tftypes.NewValue(entry.codec.Schema().Type().TerraformType(ctx), nil)

		if expected.state != nil {
			var stateSpec CaptureSpec[testConfig, testResource] = testCaptureSpec

			stateSpec.Bindings = expected.state

			stateValue, err = entry.codec.Value("movies", stateSpec)

			if err != nil {
				test.Fatal(err)
			}
		}

		var modifyPlanReq resource.ModifyPlanRequest = resource.ModifyPlanRequest{
			State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue},
			Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
			Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: configValue},
		}
		var modifyPlanResp resource.ModifyPlanResponse = resource.ModifyPlanResponse{Plan: modifyPlanReq.Plan}

		entry.ModifyPlan(ctx, modifyPlanReq, &modifyPlanResp)

		if len(backend.discovers) != expected.discovers {
			test.Errorf("%s: discovered %d times, want %d", name, len(backend.discovers), expected.discovers)
		}

		for _, spec := range backend.discovers {
			if bindings, _ := spec["bindings"].([]any); spec["endpoint"] == nil || bindings == nil || len(bindings) != 0 || spec["autoDiscover"] != nil {
				test.Errorf("%s: discovered %v, want the endpoint only with no bindings", name, spec)
			}
		}

		if expected.wantError != "" {
			if !strings.Contains(fmt.Sprint(modifyPlanResp.Diagnostics), expected.wantError) {
				test.Errorf("%s: diagnostics = %v, want %q", name, modifyPlanResp.Diagnostics, expected.wantError)
			}

			continue
		}

		if modifyPlanResp.Diagnostics.HasError() {
			test.Errorf("%s: %v", name, modifyPlanResp.Diagnostics)
			continue
		}

		planned, _, err := bindings(modifyPlanResp.Plan.Raw)

		if err != nil || len(planned) != len(expected.want) {
			test.Fatalf("%s: planned bindings = %v (%v), want %d", name, planned, err, len(expected.want))
		}

		for index, want := range expected.want {
			if target := bindingAttribute(planned[index], bindingTargetAttribute); !target.Equal(tftypes.NewValue(tftypes.String, want)) {
				test.Errorf("%s: bindings[%d].target = %v, want %v", name, index, target, want)
			}

			if bindingName := bindingAttribute(planned[index], bindingNameAttribute); !bindingName.Equal(tftypes.NewValue(tftypes.String, want)) {
				test.Errorf("%s: bindings[%d].name = %v, want the target %v", name, index, bindingName, want)
			}
		}
	}
}

// Plans pair set elements by content, so a binding added to a set must leave the others planned as in state, whatever
// the order Terraform sends the configuration, plan and state in.
func TestResourceModifyPlanPlansSetBindingsAsInState(test *testing.T) {
	var ctx context.Context = context.Background()
	var stateSpec CaptureSpec[testConfig, testResource] = testCaptureSpec
	var configSpec CaptureSpec[testConfig, testResource] = testCaptureSpec
	var planSpec CaptureSpec[testConfig, testResource] = testCaptureSpec
	var orders capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}
	var users capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "users"}, Target: "acmeCo/users"}
	var events capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "events"}, Target: "acmeCo/events"}
	var backend *testBackend = &testBackend{live: liveCapture(
		map[string]any{"resource": map[string]any{"table": "orders", "_meta": map[string]any{"path": []any{"orders"}}}, "target": "acmeCo/orders"},
		map[string]any{"resource": map[string]any{"table": "users", "_meta": map[string]any{"path": []any{"users"}}}, "target": "acmeCo/users"},
	)}

	entry := newTestCaptureEntry(backend)

	stateSpec.Bindings = []capture.Binding[testResource]{orders, users}
	configSpec.Bindings = []capture.Binding[testResource]{events, {Resource: users.Resource}, orders}
	planSpec.Bindings = []capture.Binding[testResource]{users, orders, events}

	stateValue, err := entry.codec.Value("movies", stateSpec)

	if err == nil {
		stateValue, err = withResourcePaths(stateValue, func(_ int, binding tftypes.Value) tftypes.Value {
			table, _ := known[string](resourceString(binding, "table"))

			return resourcePathValue([]string{table})
		})
	}

	configValue, configErr := entry.codec.Value("movies", configSpec)

	if configErr == nil {
		configValue, configErr = withValue(configValue, tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(1).WithAttributeName("target"), nil)
	}

	if configErr == nil {
		configValue, configErr = nullAttributes(configValue, func(path *tftypes.AttributePath) bool {
			_, isName := bindingAttributeElement(path, bindingNameAttribute)

			return isName
		})
	}

	// Terraform proposes the prior users and orders, the configured users target left unset, and an unknown name and
	// resource path for events.
	planValue, planErr := entry.codec.Value("movies", planSpec)

	if planErr == nil {
		planValue, planErr = withResourcePaths(planValue, func(index int, binding tftypes.Value) tftypes.Value {
			table, _ := known[string](resourceString(binding, "table"))

			if table == "events" {
				return tftypes.NewValue(resourcePathType, tftypes.UnknownValue)
			}

			return resourcePathValue([]string{table})
		})
	}

	if planErr == nil {
		planValue, planErr = withValue(planValue, tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(2).WithAttributeName("name"), tftypes.UnknownValue)
	}

	if err != nil || configErr != nil || planErr != nil {
		test.Fatal(err, configErr, planErr)
	}

	var modifyPlanReq resource.ModifyPlanRequest = resource.ModifyPlanRequest{
		State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue},
		Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
		Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: configValue},
	}
	var modifyPlanResp resource.ModifyPlanResponse = resource.ModifyPlanResponse{Plan: modifyPlanReq.Plan}

	entry.ModifyPlan(ctx, modifyPlanReq, &modifyPlanResp)

	if modifyPlanResp.Diagnostics.HasError() || modifyPlanResp.Diagnostics.WarningsCount() > 0 || len(backend.discovers) != 0 {
		test.Fatalf("diagnostics = %v, discovered %d times, want none", modifyPlanResp.Diagnostics, len(backend.discovers))
	}

	planned, _, err := bindings(modifyPlanResp.Plan.Raw)
	stored, _, storedErr := bindings(stateValue)

	if err != nil || storedErr != nil || len(planned) != 3 {
		test.Fatalf("planned bindings = %v (%v, %v), want 3", planned, err, storedErr)
	}

	for _, binding := range stored {
		if bindingIndex(planned, binding) == -1 {
			test.Errorf("state binding %v isn't planned as it is, so plans would show it changed", binding)
		}
	}
}
