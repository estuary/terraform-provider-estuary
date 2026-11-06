package builder

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type testWarehouseEntry = entryResource[materializationDocument[testWarehouseConfig, testWarehouseResource], MaterializationSpec[testWarehouseConfig, testWarehouseResource]]

func TestPlanResourcePathsOfMaterializations(test *testing.T) {
	var ctx context.Context = context.Background()
	var orders materialization.Binding[testWarehouseResource] = materialization.Binding[testWarehouseResource]{
		Resource: testWarehouseResource{Table: "orders", Schema: "public"},
		Source:   materialization.BindingSource{Name: "acmeCo/orders"},
	}
	var users materialization.Binding[testWarehouseResource] = materialization.Binding[testWarehouseResource]{
		Resource: testWarehouseResource{Table: "users", Schema: "public"},
		Source:   materialization.BindingSource{Name: "acmeCo/users"},
	}
	var moved materialization.Binding[testWarehouseResource] = materialization.Binding[testWarehouseResource]{
		Resource: testWarehouseResource{Table: "orders_v2", Schema: "public"},
		Source:   materialization.BindingSource{Name: "acmeCo/orders"},
	}

	expectations := map[string]struct {
		planned     []materialization.Binding[testWarehouseResource]
		created     bool
		validateErr error
		want        [][]string
		validated   bool
	}{
		"keeps the state path of an unchanged binding": {[]materialization.Binding[testWarehouseResource]{orders}, false, nil, [][]string{{"public", "orders"}}, false},
		"validates a binding whose resource changed":   {[]materialization.Binding[testWarehouseResource]{moved}, false, nil, [][]string{{"orders_v2"}}, true},
		"validates every binding beside a new one":     {[]materialization.Binding[testWarehouseResource]{orders, users}, false, nil, [][]string{{"orders"}, {"users"}}, true},
		"leaves paths unknown on a create":             {[]materialization.Binding[testWarehouseResource]{orders}, true, nil, nil, false},
		"fails when the validation fails":              {[]materialization.Binding[testWarehouseResource]{users}, false, errors.New("connector unreachable"), nil, true},
	}

	for name, expected := range expectations {
		var configureResp resource.ConfigureResponse
		var backend *testBackend = &testBackend{
			validateErr: expected.validateErr,
			live: map[string]any{
				"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com", "password_sops": "sealed"}}},
				"bindings": []any{map[string]any{"resource": map[string]any{"table": "orders", "schema": "public", "_meta": map[string]any{"path": []any{"public", "orders"}}}, "source": "acmeCo/orders", "backfill": 3}},
			},
		}

		entry := NewMaterialization[testWarehouseConfig, testWarehouseResource](testNamedConnector)().(*testWarehouseEntry)

		entry.Configure(ctx, resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/"}}, &configureResp)

		spec := func(bindings []materialization.Binding[testWarehouseResource]) MaterializationSpec[testWarehouseConfig, testWarehouseResource] {
			return MaterializationSpec[testWarehouseConfig, testWarehouseResource]{
				Endpoint: endpoint.ConnectorSpec[testWarehouseConfig]{Connector: endpoint.Connector[testWarehouseConfig]{Image: testNamedConnector.Image, Config: testWarehouseConfig{Host: "db.example.com"}}},
				Bindings: bindings,
			}
		}

		stateValue, err := entry.codec.Value("warehouse", spec([]materialization.Binding[testWarehouseResource]{orders}))

		if err == nil {
			stateValue, err = withResourcePaths(stateValue, func(int, tftypes.Value) tftypes.Value {
				return resourcePathValue([]string{"public", "orders"})
			})
		}

		if err != nil {
			test.Fatal(err)
		}

		planValue, err := entry.codec.Value("warehouse", spec(expected.planned))

		if err != nil {
			test.Fatal(err)
		}

		if expected.created {
			stateValue = tftypes.NewValue(stateValue.Type(), nil)
		}

		planned, err := entry.planResourcePaths(ctx, planValue, stateValue)

		if (err != nil) != (expected.validateErr != nil) || (len(backend.validated) > 0) != expected.validated {
			test.Errorf("%s: error = %v after %d validations, want error %v and validated %v", name, err, len(backend.validated), expected.validateErr, expected.validated)
			continue
		}

		if err != nil {
			continue
		}

		plannedBindings, _, err := bindings(planned)

		if err != nil {
			test.Fatal(err)
		}

		for index, binding := range plannedBindings {
			var current tftypes.Value = blockAttribute(binding, resourcePathAttribute, resourcePathType)
			path, isKnown := bindingResourcePath(binding)

			if expected.want == nil && current.IsKnown() || expected.want != nil && (!isKnown || !reflect.DeepEqual(path, expected.want[index])) {
				test.Errorf("%s: resource_path of binding %d = %v, want %v", name, index, current, expected.want)
			}
		}

		if !expected.validated {
			continue
		}

		var validated catalog.RawMaterialization = backend.validated[0]

		if validated[catalog.EndpointKey] == nil || !strings.Contains(fmt.Sprint(validated), "password_sops") {
			test.Errorf("%s: validated spec = %v, want the sealed live endpoint", name, validated)
		}

		for _, binding := range catalog.Bindings(validated) {
			counter, _ := binding.(map[string]any)[catalog.BackfillKey].(int)

			if want := map[string]int{"orders": 3}[catalog.BindingResource(binding)["table"].(string)]; counter != want {
				test.Errorf("%s: validated binding %v has backfill %d, want the live counter of an unchanged resource, %d", name, binding, counter, want)
			}
		}
	}
}

func TestPlanResourcePathsOfCaptures(test *testing.T) {
	var ctx context.Context = context.Background()
	var backend *testBackend = &testBackend{}
	var spec CaptureSpec[testConfig, testResource] = testCaptureSpec

	entry := newTestCaptureEntry(backend)

	spec.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}

	value, err := entry.codec.Value("movies", spec)

	if err != nil {
		test.Fatal(err)
	}

	planned, err := entry.planResourcePaths(ctx, value, tftypes.NewValue(value.Type(), nil))

	if err != nil {
		test.Fatal(err)
	}

	plannedBindings, _, err := bindings(planned)

	if err != nil {
		test.Fatal(err)
	}

	if path, _ := bindingResourcePath(plannedBindings[0]); !reflect.DeepEqual(path, []string{"orders"}) || len(backend.validated) != 0 {
		test.Errorf("resource_path = %v after %d validations, want the declared path without validating", path, len(backend.validated))
	}
}

func TestPublishedResourcePaths(test *testing.T) {
	var ctx context.Context = context.Background()
	var configureResp resource.ConfigureResponse
	var backend *testBackend = &testBackend{live: map[string]any{
		"bindings": []any{
			map[string]any{"resource": map[string]any{"table": "orders", "schema": "public", "delta_updates": false, "_meta": map[string]any{"path": []any{"public", "orders"}}}, "source": "acmeCo/orders"},
			map[string]any{"resource": map[string]any{"table": "users", "schema": "public", "_meta": map[string]any{"path": []any{"public", "users"}}}, "source": "acmeCo/users"},
			map[string]any{"resource": map[string]any{"table": "users", "schema": "public", "_meta": map[string]any{"path": []any{"PUBLIC", "users"}}}, "source": "acmeCo/users"},
		},
	}}

	entry := NewMaterialization[testWarehouseConfig, testWarehouseResource](testNamedConnector)().(*testWarehouseEntry)

	entry.Configure(ctx, resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/"}}, &configureResp)

	value, err := entry.codec.Value("warehouse", MaterializationSpec[testWarehouseConfig, testWarehouseResource]{
		Endpoint: endpoint.ConnectorSpec[testWarehouseConfig]{Connector: endpoint.Connector[testWarehouseConfig]{Image: testNamedConnector.Image, Config: testWarehouseConfig{Host: "db.example.com"}}},
		Bindings: []materialization.Binding[testWarehouseResource]{
			{Resource: testWarehouseResource{Table: "orders", Schema: "public"}, Source: materialization.BindingSource{Name: "acmeCo/orders"}},
			{Resource: testWarehouseResource{Table: "users", Schema: "public"}, Source: materialization.BindingSource{Name: "acmeCo/users"}},
		},
	})

	if err == nil {
		value, err = withResourcePaths(value, func(int, tftypes.Value) tftypes.Value {
			return tftypes.NewValue(resourcePathType, tftypes.UnknownValue)
		})
	}

	if err != nil {
		test.Fatal(err)
	}

	stored, err := entry.publishedResourcePaths(ctx, "acmeCo/warehouse/materialize-test", value)

	if err != nil {
		test.Fatal(err)
	}

	storedBindings, _, err := bindings(stored)

	if err != nil {
		test.Fatal(err)
	}

	if path, _ := bindingResourcePath(storedBindings[0]); !reflect.DeepEqual(path, []string{"public", "orders"}) {
		test.Errorf("resource_path of orders = %v, want the live one, whose resource also holds a default", path)
	}

	if current := blockAttribute(storedBindings[1], resourcePathAttribute, resourcePathType); !current.IsKnown() || !current.IsNull() {
		test.Errorf("resource_path of users = %v, want null, as two live bindings hold its resource", current)
	}
}
