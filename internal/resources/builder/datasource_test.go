package builder

import (
	"context"
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCaptureDataSourceReadsLiveBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var listed CaptureSpec[testConfig, testResource] = testCaptureSpec
	var orders capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}
	var events capture.Binding[testResource] = capture.Binding[testResource]{Resource: testResource{Table: "events"}, Target: "acmeCo/events"}

	listed.Bindings = []capture.Binding[testResource]{events, orders}

	expectations := map[string]struct {
		live  map[string]any
		fails bool
	}{
		"lists every live binding": {
			liveCapture(map[string]any{"resource": map[string]any{"table": "events"}, "target": "acmeCo/events"}, map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"}),
			false,
		},
		"fails on a missing entry rather than listing none": {nil, true},
	}

	for name, expected := range expectations {
		var configureResp datasource.ConfigureResponse
		var schemaResp datasource.SchemaResponse
		var backend *testBackend = &testBackend{live: expected.live}

		dataSource := NewCaptureDataSource[testConfig, testResource](testNamedConnector)().(*entryDataSource[captureDocument[testConfig, testResource], CaptureSpec[testConfig, testResource]])

		dataSource.Configure(ctx, datasource.ConfigureRequest{ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/"}}, &configureResp)
		dataSource.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

		if schemaResp.Diagnostics.HasError() {
			test.Fatalf("%s: schema: %v", name, schemaResp.Diagnostics)
		}

		var stateType tftypes.Object = schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)

		var config tftypes.Value = tftypes.NewValue(stateType, map[string]tftypes.Value{
			nameAttribute:         tftypes.NewValue(tftypes.String, "movies"),
			liveBindingsAttribute: tftypes.NewValue(stateType.AttributeTypes[liveBindingsAttribute], nil),
		})

		var readResp datasource.ReadResponse = datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

		dataSource.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config}}, &readResp)

		if !reflect.DeepEqual(backend.reads, []string{"acmeCo/movies/source-test"}) {
			test.Errorf("%s: read specs of %v, want the catalog name", name, backend.reads)
		}

		if readResp.Diagnostics.HasError() != expected.fails {
			test.Errorf("%s: read diagnostics = %v, want failure %v", name, readResp.Diagnostics, expected.fails)
			continue
		}

		if expected.fails {
			continue
		}

		listedValue, err := newTestCaptureEntry(backend).codec.Value("movies", listed)

		if err != nil {
			test.Fatal(err)
		}

		want, err := withLiveBindings(config, listedValue)

		if err != nil {
			test.Fatal(err)
		}

		if !readResp.State.Raw.Equal(want) {
			test.Errorf("%s: state = %v, want %v", name, readResp.State.Raw, want)
		}
	}
}
