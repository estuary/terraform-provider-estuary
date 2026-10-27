package builder

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func bindingTargetPath(index int) *tftypes.AttributePath {
	return tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(index).WithAttributeName("target")
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
			wantError: "discovery found no resource for bindings [1]",
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

		for index, want := range expected.want {
			target, err := attributeValue(modifyPlanResp.Plan.Raw, bindingTargetPath(index))

			if err != nil || !target.Equal(tftypes.NewValue(tftypes.String, want)) {
				test.Errorf("%s: bindings[%d].target = %v (%v), want %v", name, index, target, err, want)
			}

			bindingName, err := attributeValue(modifyPlanResp.Plan.Raw, tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(index).WithAttributeName("name"))

			if err != nil || !bindingName.Equal(tftypes.NewValue(tftypes.String, want)) {
				test.Errorf("%s: bindings[%d].name = %v (%v), want the target %v", name, index, bindingName, err, want)
			}
		}
	}
}
