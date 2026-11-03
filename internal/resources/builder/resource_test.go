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
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var testNamedConnector Connector = Connector{
	TypeName:      "source_test",
	Image:         "ghcr.io/estuary/source-test:v1",
	ConnectorName: "source-test",
}

var testCaptureSpec CaptureSpec[testConfig, testResource] = CaptureSpec[testConfig, testResource]{
	Endpoint: endpoint.ConnectorSpec[testConfig]{
		Connector: endpoint.Connector[testConfig]{
			Image:  testNamedConnector.Image,
			Config: testConfig{Host: "db.example.com"},
		},
	},
}

type testCaptureEntry = entryResource[captureDocument[testConfig, testResource], CaptureSpec[testConfig, testResource]]

// newTestCaptureEntry returns a capture resource using backend under the acmeCo/ tenant.
func newTestCaptureEntry(backend *testBackend) *testCaptureEntry {
	var configureResp resource.ConfigureResponse

	entry := NewCapture[testConfig, testResource](testNamedConnector)().(*testCaptureEntry)

	entry.Configure(context.Background(), resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/"}}, &configureResp)

	return entry
}

// withValue returns value with the attribute at path set to raw, nil being null.
func withValue(value tftypes.Value, path *tftypes.AttributePath, raw any) (tftypes.Value, error) {
	return withAttribute(value, path, func(current tftypes.Value) tftypes.Value {
		return tftypes.NewValue(current.Type(), raw)
	})
}

// testBackend is an in-memory catalog.Backend recording read, deleted and published specs, failing publications with
// publishErr.
type testBackend struct {
	live       map[string]any
	reads      []string
	deletes    []string
	published  map[string]any
	discovered map[string]map[string]any
	discovers  []catalog.RawCapture
	discover   bool
	resets     []string
	publishErr error
}

func (backend *testBackend) PublishCapture(_ context.Context, name string, spec catalog.CaptureSpec, resets []string) error {
	backend.resets = resets

	backend.published = spec.(catalog.RawCapture)

	return backend.publishErr
}

func (backend *testBackend) PublishDiscoveredCapture(_ context.Context, name string, spec catalog.CaptureSpec, _ catalog.BindingsIdentifier) error {
	backend.published = spec.(catalog.RawCapture)
	backend.discover = true

	return backend.publishErr
}

func (backend *testBackend) PublishMaterialization(_ context.Context, name string, spec catalog.MaterializationSpec) error {
	backend.published = spec.(catalog.RawMaterialization)

	return backend.publishErr
}

func (backend *testBackend) DeleteCatalogEntry(_ context.Context, name string) error {
	backend.deletes = append(backend.deletes, name)

	return nil
}

func (backend *testBackend) DiscoverCapture(_ context.Context, _ string, spec catalog.CaptureSpec) (map[string]map[string]any, error) {
	backend.discovers = append(backend.discovers, spec.(catalog.RawCapture))

	return backend.discovered, nil
}

func (backend *testBackend) ReadSpec(_ context.Context, name string) (map[string]any, error) {
	backend.reads = append(backend.reads, name)

	return backend.live, nil
}

func TestResourceUpdateMergesLiveBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var updateResp resource.UpdateResponse
	var discovered map[string]any = map[string]any{"resource": map[string]any{"table": "events"}, "target": "acmeCo/events", "notBefore": "2026-01-01"}
	var backend *testBackend = &testBackend{
		live: map[string]any{
			"bindings": []any{
				discovered,
				map[string]any{"resource": map[string]any{"table": "users"}, "target": "acmeCo/users"},
				map[string]any{"resource": map[string]any{"table": "logs"}, "target": "acmeCo/logs"},
			},
		},
	}

	entry := newTestCaptureEntry(backend)

	var prior CaptureSpec[testConfig, testResource] = testCaptureSpec
	var planned CaptureSpec[testConfig, testResource] = testCaptureSpec

	prior.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true}
	prior.Bindings = []capture.Binding[testResource]{
		{Resource: testResource{Table: "users"}, Target: "acmeCo/users"},
		{Resource: testResource{Table: "logs"}, Target: "acmeCo/logs"},
	}

	planned.AutoDiscover = prior.AutoDiscover
	planned.Bindings = []capture.Binding[testResource]{
		{Resource: testResource{Table: "users", Priority: 2}, Target: "acmeCo/users"},
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

	updateResp.State = tfsdk.State{Schema: entry.codec.Schema()}

	entry.Update(ctx, resource.UpdateRequest{
		State: tfsdk.State{Schema: entry.codec.Schema(), Raw: priorValue},
		Plan:  tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
	}, &updateResp)

	if updateResp.Diagnostics.HasError() {
		test.Fatalf("update: %v", updateResp.Diagnostics)
	}

	var want []any = []any{
		discovered,
		map[string]any{"resource": map[string]any{"table": "users", "priority": 2}, "target": "acmeCo/users"},
		map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"},
	}

	if !reflect.DeepEqual(backend.reads, []string{"acmeCo/movies/source-test"}) {
		test.Errorf("read specs of %v, want the catalog name", backend.reads)
	}

	if !reflect.DeepEqual(backend.published["bindings"], want) {
		test.Errorf("published bindings = %v, want %v", backend.published["bindings"], want)
	}

	if !updateResp.State.Raw.Equal(planValue) {
		test.Error("state differs from the plan, want only the planned bindings in state")
	}
}

func TestResourcePublishesPlannedBindingsWithoutMerge(test *testing.T) {
	var ctx context.Context = context.Background()
	var discovering CaptureSpec[testConfig, testResource] = testCaptureSpec
	var plain CaptureSpec[testConfig, testResource] = testCaptureSpec

	discovering.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true}
	discovering.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}
	plain.Bindings = discovering.Bindings

	expectations := map[string]struct {
		spec     CaptureSpec[testConfig, testResource]
		create   bool
		reads    int
		discover bool
	}{
		"create with auto discover":    {discovering, true, 0, true},
		"create without auto discover": {plain, true, 0, false},
		"update without auto discover": {plain, false, 1, false},
	}

	for name, expected := range expectations {
		var backend *testBackend = &testBackend{live: map[string]any{"bindings": []any{map[string]any{"target": "acmeCo/events"}}}}

		entry := newTestCaptureEntry(backend)

		value, err := entry.codec.Value("movies", expected.spec)

		if err == nil && expected.spec.AutoDiscover == nil {
			value, err = withValue(value, keepUnmanagedBindingsPath, false)
		}

		if err != nil {
			test.Fatal(err)
		}

		var plan tfsdk.Plan = tfsdk.Plan{Schema: entry.codec.Schema(), Raw: value}
		var state tfsdk.State = tfsdk.State{Schema: entry.codec.Schema(), Raw: value}

		if expected.create {
			var createResp resource.CreateResponse = resource.CreateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

			entry.Create(ctx, resource.CreateRequest{Plan: plan}, &createResp)
		} else {
			var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

			entry.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &updateResp)
		}

		var want []any = []any{map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"}}

		if len(backend.reads) != expected.reads || !reflect.DeepEqual(backend.published["bindings"], want) {
			test.Errorf("%s: read %v and published %v, want %d reads and only the planned bindings", name, backend.reads, backend.published["bindings"], expected.reads)
		}

		if backend.discover != expected.discover {
			test.Errorf("%s: published with discovery %v, want %v", name, backend.discover, expected.discover)
		}
	}
}

func TestResourceModifyPlanSecretsHash(test *testing.T) {
	var ctx context.Context = context.Background()
	var priorSpec CaptureSpec[testConfig, testResource] = testCaptureSpec

	entry := newTestCaptureEntry(&testBackend{})

	priorSpec.Endpoint.Connector.Config.Password = "secret"

	priorValue, err := entry.codec.Value("movies", priorSpec)

	if err == nil {
		priorValue, err = entry.codec.secrets.nullSecrets(priorValue)
	}

	if err != nil {
		test.Fatal(err)
	}

	stored, err := hashSecret("secret")

	if err != nil {
		test.Fatal(err)
	}

	expectations := map[string]struct {
		password string
		unknown  bool
		create   bool
		stored   bool
		want     any
		warn     bool
	}{
		"keeps the hash of an unchanged secret":   {password: "secret", stored: true, want: stored},
		"plans and warns about a changed secret":  {password: "rotated", stored: true, want: tftypes.UnknownValue, warn: true},
		"plans a secret without a hash":           {password: "secret", want: tftypes.UnknownValue},
		"plans the secret of a created entry":     {password: "secret", create: true, want: tftypes.UnknownValue},
		"plans a secret not known yet":            {unknown: true, stored: true, want: tftypes.UnknownValue},
		"plans the null hash of a removed secret": {stored: true},
	}

	for name, expected := range expectations {
		var plannedSpec CaptureSpec[testConfig, testResource] = testCaptureSpec

		plannedSpec.Endpoint.Connector.Config.Password = expected.password

		configValue, err := entry.codec.Value("movies", plannedSpec)

		if err != nil {
			test.Fatal(err)
		}

		if expected.unknown {
			configValue, err = withValue(configValue, testPasswordPath, tftypes.UnknownValue)
		}

		if err != nil {
			test.Fatal(err)
		}

		planValue, err := entry.codec.secrets.nullSecrets(configValue)

		if err != nil {
			test.Fatal(err)
		}

		var stateValue tftypes.Value = priorValue

		if expected.stored {
			stateValue, err = withValue(priorValue, testPasswordHashPath, stored)
		}

		if expected.create {
			stateValue = tftypes.NewValue(entry.codec.Schema().Type().TerraformType(ctx), nil)
		}

		if err != nil {
			test.Fatal(err)
		}

		var modifyPlanReq resource.ModifyPlanRequest = resource.ModifyPlanRequest{
			State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue},
			Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
			Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: configValue},
		}
		var modifyPlanResp resource.ModifyPlanResponse = resource.ModifyPlanResponse{Plan: modifyPlanReq.Plan}

		entry.ModifyPlan(ctx, modifyPlanReq, &modifyPlanResp)

		if modifyPlanResp.Diagnostics.HasError() {
			test.Errorf("%s: %v", name, modifyPlanResp.Diagnostics)
			continue
		}

		hash, err := attributeValue(modifyPlanResp.Plan.Raw, testPasswordHashPath)

		if err != nil {
			test.Fatal(err)
		}

		if want := tftypes.NewValue(tftypes.String, expected.want); !hash.Equal(want) {
			test.Errorf("%s: planned password_hash = %v, want %v", name, hash, want)
		}

		var warnings diag.Diagnostics = modifyPlanResp.Diagnostics.Warnings()

		if warned := len(warnings) == 1; warned != expected.warn || len(warnings) > 1 {
			test.Errorf("%s: warnings = %v, want a changed secret warning %v", name, warnings, expected.warn)
		}

		if expected.warn {
			warning, isAttribute := warnings[0].(diag.DiagnosticWithPath)

			if !isAttribute || !warning.Path().Equal(path.Root("config").AtName("password")) || !strings.Contains(warning.Detail(), "config.password") {
				test.Errorf("%s: warning = %v, want it at config.password", name, warnings[0])
			}
		}
	}
}

func TestResourceUpdateKeepsUnchangedSecrets(test *testing.T) {
	var ctx context.Context = context.Background()
	var priorSpec CaptureSpec[testConfig, testResource] = testCaptureSpec
	var sealed map[string]any = map[string]any{
		"endpoint": map[string]any{
			"connector": map[string]any{
				"image":  testNamedConnector.Image,
				"config": map[string]any{"host": "db.example.com", "password_sops": "ENC[AES256_GCM,data:abc,type:str]", "sops": map[string]any{"mac": "<encrypted>"}},
			},
		},
		"bindings": []any{},
	}

	priorSpec.Endpoint.Connector.Config.Password = "secret"

	stored, err := hashSecret(priorSpec.Endpoint.Connector.Config.Password)

	if err != nil {
		test.Fatal(err)
	}

	var sealedWithExtras map[string]any = map[string]any{
		"endpoint": map[string]any{
			"connector": map[string]any{
				"image":  testNamedConnector.Image,
				"config": map[string]any{"host": "db.example.com", "ratio": 0.5, "unmodeled": true, "password_sops": "ENC[AES256_GCM,data:abc,type:str]", "sops": map[string]any{"mac": "<encrypted>"}},
			},
		},
		"bindings": []any{},
	}

	var sealedOtherImage map[string]any = map[string]any{
		"endpoint": map[string]any{
			"connector": map[string]any{
				"image":  "ghcr.io/estuary/source-other:v1",
				"config": map[string]any{"host": "db.example.com", "password_sops": "ENC[AES256_GCM,data:abc,type:str]", "sops": map[string]any{"mac": "<encrypted>"}},
			},
		},
		"bindings": []any{},
	}

	expectations := map[string]struct {
		live      map[string]any
		host      string
		password  string
		sealed    bool
		plaintext map[string]any
	}{
		"publishes the planned image over another live one":        {sealedOtherImage, "db.example.com", "secret", false, map[string]any{"host": "db.example.com", "password": "secret"}},
		"keeps the sealed endpoint of an unchanged config":         {sealed, "db.example.com", "secret", true, nil},
		"keeps the sealed endpoint holding fields Terraform skips": {sealedWithExtras, "db.example.com", "secret", true, nil},
		"publishes a changed config":                               {sealed, "other.example.com", "secret", false, map[string]any{"host": "other.example.com", "password": "secret"}},
		"publishes changed secrets":                                {sealed, "db.example.com", "rotated", false, map[string]any{"host": "db.example.com", "password": "rotated"}},
		"publishes removed secrets":                                {sealed, "db.example.com", "", false, map[string]any{"host": "db.example.com"}},
	}

	for name, expected := range expectations {
		var backend *testBackend = &testBackend{live: expected.live}
		var plannedSpec CaptureSpec[testConfig, testResource] = testCaptureSpec

		entry := newTestCaptureEntry(backend)

		plannedSpec.Endpoint.Connector.Config = testConfig{Host: expected.host, Password: expected.password}
		plannedSpec.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}

		priorValue, err := entry.codec.Value("movies", priorSpec)

		if err != nil {
			test.Fatal(err)
		}

		configValue, err := entry.codec.Value("movies", plannedSpec)

		if err != nil {
			test.Fatal(err)
		}

		stateValue, err := entry.codec.secrets.nullSecrets(priorValue)

		if err == nil {
			stateValue, err = withValue(stateValue, testPasswordHashPath, stored)
		}

		if err != nil {
			test.Fatal(err)
		}

		planValue, err := entry.codec.secrets.nullSecrets(configValue)

		if err == nil {
			planValue, _, err = entry.codec.secrets.planHashes(planValue, configValue, stateValue)
		}

		if err != nil {
			test.Fatal(err)
		}

		var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

		entry.Update(ctx, resource.UpdateRequest{
			State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue},
			Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
			Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: configValue},
		}, &updateResp)

		if updateResp.Diagnostics.HasError() {
			test.Errorf("%s: update: %v", name, updateResp.Diagnostics)
			continue
		}

		if expected.sealed && !reflect.DeepEqual(backend.published["endpoint"], expected.live["endpoint"]) {
			test.Errorf("%s: published endpoint = %v, want the sealed live endpoint", name, backend.published["endpoint"])
		}

		if config := utils.Lookup(backend.published, "endpoint", "connector", "config"); !expected.sealed && !reflect.DeepEqual(config, expected.plaintext) {
			test.Errorf("%s: published config = %v, want the planned config with its secrets", name, config)
		}

		if password, _ := attributeValue(updateResp.State.Raw, testPasswordPath); !password.IsNull() {
			test.Errorf("%s: state password = %v, want none in state", name, password)
		}

		hash, err := attributeValue(updateResp.State.Raw, testPasswordHashPath)

		if err != nil {
			test.Fatal(err)
		}

		var text string

		if expected.password == "" && !hash.IsNull() {
			test.Errorf("%s: state password_hash = %v, want null for the removed secret", name, hash)
		}

		if expected.password != "" && (hash.As(&text) != nil || !verifySecret(expected.password, text)) {
			test.Errorf("%s: state password_hash = %v, want the hash of the planned password", name, hash)
		}
	}
}

func TestResourceModifyPlanReplacesOnCatalogNameChange(test *testing.T) {
	var ctx context.Context = context.Background()

	entry := newTestCaptureEntry(&testBackend{})

	expectations := map[string]struct {
		state   string
		plan    string
		stored  string
		replace bool
		warn    bool
	}{
		"unchanged name":                 {"movies", "movies", "", false, false},
		"adds the tenant":                {"movies", "acmeCo/movies", "", false, false},
		"adds the connector name":        {"movies", "movies/source-test", "", false, false},
		"writes the catalog name":        {"movies", "acmeCo/movies/source-test", "", false, false},
		"drops the full catalog name":    {"acmeCo/movies/source-test", "movies", "", false, false},
		"renames the entry":              {"movies", "films", "", true, false},
		"moves to another tenant":        {"movies", "otherCo/movies", "", true, false},
		"keeps the stored catalog name":  {"movies", "movies", "acmeCo/movies/source-test", false, false},
		"renames a stored entry":         {"movies", "films", "acmeCo/movies/source-test", true, false},
		"moves with the provider tenant": {"movies", "movies", "oldCo/movies/source-test", true, true},
	}

	for name, expected := range expectations {
		stateValue, err := entry.codec.Value(expected.state, testCaptureSpec)

		if err == nil && expected.stored != "" {
			stateValue, err = withCatalogName(stateValue, expected.stored)
		}

		if err != nil {
			test.Fatal(err)
		}

		planValue, err := entry.codec.Value(expected.plan, testCaptureSpec)

		if err != nil {
			test.Fatal(err)
		}

		var modifyPlanReq resource.ModifyPlanRequest = resource.ModifyPlanRequest{
			State: tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue},
			Plan:  tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
		}
		var modifyPlanResp resource.ModifyPlanResponse = resource.ModifyPlanResponse{Plan: modifyPlanReq.Plan}

		entry.ModifyPlan(ctx, modifyPlanReq, &modifyPlanResp)

		if modifyPlanResp.Diagnostics.HasError() {
			test.Errorf("%s: %v", name, modifyPlanResp.Diagnostics)
			continue
		}

		if replace := len(modifyPlanResp.RequiresReplace) > 0; replace != expected.replace {
			test.Errorf("%s: %s -> %s requires replace = %v, want %v", name, expected.state, expected.plan, replace, expected.replace)
		}

		var warnings diag.Diagnostics = modifyPlanResp.Diagnostics.Warnings()

		if warned := len(warnings) == 1 && strings.HasPrefix(warnings[0].Summary(), "Provider tenant changed: oldCo/movies/source-test"); warned != expected.warn {
			test.Errorf("%s: warnings = %v, want a tenant change warning %v", name, warnings, expected.warn)
		}

		var planned string = entry.catalogName(expected.plan)

		if got := blockAttribute(modifyPlanResp.Plan.Raw, catalogNameAttribute, tftypes.String); !got.Equal(tftypes.NewValue(tftypes.String, planned)) {
			test.Errorf("%s: planned catalog_name = %v, want %s", name, got, planned)
		}
	}
}

func TestResourceAddressesTheStoredCatalogName(test *testing.T) {
	var ctx context.Context = context.Background()
	var backend *testBackend = &testBackend{live: liveCapture()}

	entry := newTestCaptureEntry(backend)

	stateValue, err := entry.codec.Value("movies", testCaptureSpec)

	if err == nil {
		stateValue, err = withCatalogName(stateValue, "oldCo/movies/source-test")
	}

	if err != nil {
		test.Fatal(err)
	}

	var state tfsdk.State = tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}
	var readResp resource.ReadResponse = resource.ReadResponse{State: state}
	var deleteResp resource.DeleteResponse

	entry.Read(ctx, resource.ReadRequest{State: state}, &readResp)
	entry.Delete(ctx, resource.DeleteRequest{State: state}, &deleteResp)

	if readResp.Diagnostics.HasError() || deleteResp.Diagnostics.HasError() {
		test.Fatalf("read: %v, delete: %v", readResp.Diagnostics, deleteResp.Diagnostics)
	}

	if !reflect.DeepEqual(backend.reads, []string{"oldCo/movies/source-test"}) || !reflect.DeepEqual(backend.deletes, []string{"oldCo/movies/source-test"}) {
		test.Errorf("read %v and deleted %v, want the stored catalog name, not the acmeCo/ tenant one", backend.reads, backend.deletes)
	}
}

func TestResourceStoresTheCatalogName(test *testing.T) {
	var ctx context.Context = context.Background()
	var want tftypes.Value = tftypes.NewValue(tftypes.String, "acmeCo/movies/source-test")

	entry := newTestCaptureEntry(&testBackend{live: liveCapture()})

	legacy, err := entry.codec.Value("movies", testCaptureSpec)

	if err != nil {
		test.Fatal(err)
	}

	var readResp resource.ReadResponse = resource.ReadResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: legacy}}

	entry.Read(ctx, resource.ReadRequest{State: readResp.State}, &readResp)

	var importResp resource.ImportStateResponse = resource.ImportStateResponse{
		State: tfsdk.State{Schema: entry.codec.Schema(), Raw: tftypes.NewValue(entry.codec.Schema().Type().TerraformType(ctx), nil)},
	}

	entry.ImportState(ctx, resource.ImportStateRequest{ID: "acmeCo/movies/source-test"}, &importResp)

	if readResp.Diagnostics.HasError() || importResp.Diagnostics.HasError() {
		test.Fatalf("read: %v, import: %v", readResp.Diagnostics, importResp.Diagnostics)
	}

	if got := blockAttribute(readResp.State.Raw, catalogNameAttribute, tftypes.String); !got.Equal(want) {
		test.Errorf("refreshed catalog_name = %v, want %v filled in for state written before it existed", got, want)
	}

	if got := blockAttribute(importResp.State.Raw, catalogNameAttribute, tftypes.String); !got.Equal(want) {
		test.Errorf("imported catalog_name = %v, want %v", got, want)
	}
}

func TestResourceImportStateReadsLiveSpec(test *testing.T) {
	var ctx context.Context = context.Background()
	var discovering CaptureSpec[testConfig, testResource] = testCaptureSpec
	var managed CaptureSpec[testConfig, testResource] = testCaptureSpec
	var sharded CaptureSpec[testConfig, testResource] = testCaptureSpec

	discovering.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true, EvolveIncompatibleCollections: true}
	managed.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "users"}, Target: "acmeCo/users"}}
	sharded.Shards = catalog.Shards{"disable": true, "logLevel": "debug", "hotStandbys": int64(1)}

	expectations := map[string]struct {
		live map[string]any
		want CaptureSpec[testConfig, testResource]
		warn bool
	}{
		"leaves discovered bindings unmanaged": {
			live: map[string]any{
				"autoDiscover": map[string]any{"addNewBindings": true, "evolveIncompatibleCollections": true},
				"endpoint":     map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com"}}},
				"bindings":     []any{map[string]any{"resource": map[string]any{"table": "users"}, "target": "acmeCo/users"}},
			},
			want: discovering,
		},
		"imports managed bindings without their metadata": {
			live: map[string]any{
				"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com"}}},
				"bindings": []any{map[string]any{"resource": map[string]any{"table": "users", "_meta": map[string]any{"path": []any{"users"}}}, "target": "acmeCo/users"}},
			},
			want: managed,
		},
		"warns about unmanaged fields": {
			live: map[string]any{
				"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com"}}},
				"bindings": []any{},
				"interval": "5m",
			},
			want: testCaptureSpec,
			warn: true,
		},
		"imports the shard settings": {
			live: map[string]any{
				"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com"}}},
				"bindings": []any{},
				"shards":   map[string]any{"disable": true, "logLevel": "debug", "hotStandbys": 1},
			},
			want: sharded,
		},
	}

	for name, expected := range expectations {
		var backend *testBackend = &testBackend{live: expected.live}

		entry := newTestCaptureEntry(backend)

		var importResp resource.ImportStateResponse = resource.ImportStateResponse{
			State: tfsdk.State{
				Schema: entry.codec.Schema(),
				Raw:    tftypes.NewValue(entry.codec.Schema().Type().TerraformType(ctx), nil),
			},
		}

		entry.ImportState(ctx, resource.ImportStateRequest{ID: "acmeCo/movies/source-test"}, &importResp)

		if importResp.Diagnostics.HasError() {
			test.Errorf("%s: import: %v", name, importResp.Diagnostics)
			continue
		}

		want, err := entry.codec.Value("acmeCo/movies", expected.want)

		if err != nil {
			test.Fatal(err)
		}

		if !reflect.DeepEqual(backend.reads, []string{"acmeCo/movies/source-test"}) {
			test.Errorf("%s: read specs of %v, want the import ID", name, backend.reads)
		}

		stateWithoutLive, err := nullAttributes(importResp.State.Raw, computedByRead)

		if err != nil {
			test.Fatal(err)
		}

		if !stateWithoutLive.Equal(want) {
			test.Errorf("%s: imported state = %v, want %v", name, importResp.State.Raw, want)
		}

		if warned := importResp.Diagnostics.WarningsCount() > 0; warned != expected.warn {
			test.Errorf("%s: warned = %v, want %v", name, warned, expected.warn)
		}
	}
}

func TestResourceImportStateRejectsMissingEntry(test *testing.T) {
	var ctx context.Context = context.Background()

	entry := newTestCaptureEntry(&testBackend{})

	var importResp resource.ImportStateResponse = resource.ImportStateResponse{
		State: tfsdk.State{
			Schema: entry.codec.Schema(),
			Raw:    tftypes.NewValue(entry.codec.Schema().Type().TerraformType(ctx), nil),
		},
	}

	entry.ImportState(ctx, resource.ImportStateRequest{ID: "acmeCo/missing/source-test"}, &importResp)

	if !importResp.Diagnostics.HasError() {
		test.Error("ImportState accepted a catalog entry that doesn't exist")
	}
}

func TestResourceImportStateKeepsNamesEndingInTheConnectorName(test *testing.T) {
	var ctx context.Context = context.Background()
	var backend *testBackend = &testBackend{live: liveCapture()}
	var name types.String

	entry := newTestCaptureEntry(backend)

	var importResp resource.ImportStateResponse = resource.ImportStateResponse{
		State: tfsdk.State{
			Schema: entry.codec.Schema(),
			Raw:    tftypes.NewValue(entry.codec.Schema().Type().TerraformType(ctx), nil),
		},
	}

	entry.ImportState(ctx, resource.ImportStateRequest{ID: "acmeCo/source-test"}, &importResp)

	if importResp.Diagnostics.HasError() {
		test.Fatalf("import: %v", importResp.Diagnostics)
	}

	importResp.Diagnostics.Append(importResp.State.GetAttribute(ctx, path.Root(nameAttribute), &name)...)

	if !reflect.DeepEqual(backend.reads, []string{"acmeCo/source-test"}) || name.ValueString() != "acmeCo/source-test" {
		test.Errorf("read specs of %v into name %q, want the import ID for both", backend.reads, name.ValueString())
	}
}

func TestResourcePublishAssertsThePublicationRead(test *testing.T) {
	var ctx context.Context = context.Background()
	var mismatch error = fmt.Errorf("expected publication ID was not matched: %w", catalog.ErrPublicationConflict)
	var buildFailed error = errors.New("failed with status: buildFailed")
	var planned CaptureSpec[testConfig, testResource] = testCaptureSpec

	planned.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}

	expectations := map[string]struct {
		create     bool
		publishErr error
		want       any
		hint       string
	}{
		"create asserts no entry has the name":       {create: true, want: catalog.NewEntryPubID},
		"create failure suggests an import":          {create: true, publishErr: mismatch, want: catalog.NewEntryPubID, hint: "terraform import"},
		"update asserts the live publication":        {create: false, want: "10:a5:1e:b0:27:dc:fe:5a"},
		"update failure suggests planning once more": {create: false, publishErr: mismatch, want: "10:a5:1e:b0:27:dc:fe:5a", hint: "run terraform plan and apply again"},
		"create build failure suggests nothing":      {create: true, publishErr: buildFailed, want: catalog.NewEntryPubID},
		"update build failure suggests nothing":      {create: false, publishErr: buildFailed, want: "10:a5:1e:b0:27:dc:fe:5a"},
	}

	for name, expected := range expectations {
		var live map[string]any = liveCapture()
		var diagnostics diag.Diagnostics

		live[catalog.ExpectPubIDKey] = "10:a5:1e:b0:27:dc:fe:5a"

		var backend *testBackend = &testBackend{live: live, publishErr: expected.publishErr}

		entry := newTestCaptureEntry(backend)

		value, err := entry.codec.Value("movies", planned)

		if err == nil {
			value, err = withValue(value, keepUnmanagedBindingsPath, false)
		}

		if err != nil {
			test.Fatal(err)
		}

		var plan tfsdk.Plan = tfsdk.Plan{Schema: entry.codec.Schema(), Raw: value}

		if expected.create {
			var createResp resource.CreateResponse = resource.CreateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

			entry.Create(ctx, resource.CreateRequest{Plan: plan}, &createResp)

			diagnostics = createResp.Diagnostics
		} else {
			var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

			entry.Update(ctx, resource.UpdateRequest{Plan: plan, State: tfsdk.State{Schema: entry.codec.Schema(), Raw: value}}, &updateResp)

			diagnostics = updateResp.Diagnostics
		}

		if got := backend.published[catalog.ExpectPubIDKey]; got != expected.want {
			test.Errorf("%s: published expectPubId = %v, want %v", name, got, expected.want)
		}

		if failed := diagnostics.HasError(); failed != (expected.publishErr != nil) {
			test.Errorf("%s: failed = %v, want %v: %v", name, failed, expected.publishErr != nil, diagnostics)
		}

		if expected.publishErr != nil && expected.hint != "" && !strings.Contains(diagnostics.Errors()[0].Detail(), expected.hint) {
			test.Errorf("%s: error = %q, want it to say %q", name, diagnostics.Errors()[0].Detail(), expected.hint)
		}

		if detail := diagnostics.Errors(); expected.publishErr != nil && expected.hint == "" && detail[0].Detail() != expected.publishErr.Error() {
			test.Errorf("%s: error = %q, want only the publication error", name, detail[0].Detail())
		}
	}
}

func TestResourceKeepsThePublicationIDOutOfState(test *testing.T) {
	var ctx context.Context = context.Background()
	var live map[string]any = liveCapture()

	live[catalog.ExpectPubIDKey] = "10:a5:1e:b0:27:dc:fe:5a"

	entry := newTestCaptureEntry(&testBackend{live: live})

	stateValue, err := entry.codec.Value("movies", testCaptureSpec)

	if err != nil {
		test.Fatal(err)
	}

	var readResp resource.ReadResponse = resource.ReadResponse{State: tfsdk.State{Schema: entry.codec.Schema(), Raw: stateValue}}
	var importResp resource.ImportStateResponse = resource.ImportStateResponse{
		State: tfsdk.State{Schema: entry.codec.Schema(), Raw: tftypes.NewValue(entry.codec.Schema().Type().TerraformType(ctx), nil)},
	}

	entry.Read(ctx, resource.ReadRequest{State: readResp.State}, &readResp)
	entry.ImportState(ctx, resource.ImportStateRequest{ID: "acmeCo/movies/source-test"}, &importResp)

	if readResp.Diagnostics.HasError() || readResp.Diagnostics.WarningsCount() > 0 {
		test.Errorf("read: %v, want the publication ID left out quietly", readResp.Diagnostics)
	}

	if importResp.Diagnostics.HasError() || importResp.Diagnostics.WarningsCount() > 0 {
		test.Errorf("import: %v, want no unmanaged field warning for the publication ID", importResp.Diagnostics)
	}
}

func TestResourceConfigureRejectsUnknownProviderData(test *testing.T) {
	var configureResp resource.ConfigureResponse

	NewCapture[testConfig, testResource](testNamedConnector)().(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: "backend"}, &configureResp)

	if !configureResp.Diagnostics.HasError() {
		test.Error("Configure accepted provider data that is not a *catalog.Session")
	}
}

func TestResourceUpdateKeepsUnmanagedLiveSettings(test *testing.T) {
	var ctx context.Context = context.Background()

	expectations := map[string]struct {
		prior catalog.Shards
		plan  catalog.Shards
		want  any
	}{
		"keeps the live shards Terraform never set": {nil, nil, map[string]any{"disable": true}},
		"publishes the planned shards":              {nil, catalog.Shards{"logLevel": "warn"}, map[string]any{"logLevel": "warn"}},
		"removes the shards the plan removes":       {catalog.Shards{"logLevel": "debug"}, nil, nil},
	}

	for name, expected := range expectations {
		var prior CaptureSpec[testConfig, testResource] = testCaptureSpec
		var planned CaptureSpec[testConfig, testResource] = testCaptureSpec
		var backend *testBackend = &testBackend{live: map[string]any{
			"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com"}}},
			"bindings": []any{},
			"shards":   map[string]any{"disable": true},
		}}

		entry := newTestCaptureEntry(backend)

		prior.Shards = expected.prior
		planned.Shards = expected.plan
		planned.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}

		priorValue, err := entry.codec.Value("movies", prior)

		if err != nil {
			test.Fatal(err)
		}

		planValue, err := entry.codec.Value("movies", planned)

		if err != nil {
			test.Fatal(err)
		}

		var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

		entry.Update(ctx, resource.UpdateRequest{
			State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: priorValue},
			Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: planValue},
			Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: planValue},
		}, &updateResp)

		if updateResp.Diagnostics.HasError() {
			test.Fatalf("%s: update: %v", name, updateResp.Diagnostics)
		}

		if shards, published := backend.published["shards"]; (expected.want == nil && published) || (expected.want != nil && !reflect.DeepEqual(shards, expected.want)) {
			test.Errorf("%s: published shards = %v, want %v", name, shards, expected.want)
		}
	}
}

func TestResourceUpdateKeepsResourceMetadata(test *testing.T) {
	var ctx context.Context = context.Background()
	var spec CaptureSpec[testConfig, testResource] = testCaptureSpec
	var backend *testBackend = &testBackend{live: map[string]any{
		"endpoint": map[string]any{"connector": map[string]any{"image": testNamedConnector.Image, "config": map[string]any{"host": "db.example.com"}}},
		"bindings": []any{
			map[string]any{"resource": map[string]any{"table": "users", "_meta": map[string]any{"path": []any{"users"}}}, "target": "acmeCo/users"},
		},
	}}

	entry := newTestCaptureEntry(backend)

	spec.Bindings = []capture.Binding[testResource]{
		{Resource: testResource{Table: "users", Priority: 2}, Target: "acmeCo/users"},
		{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"},
	}

	value, err := entry.codec.Value("movies", spec)

	if err != nil {
		test.Fatal(err)
	}

	var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

	entry.Update(ctx, resource.UpdateRequest{
		State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: value},
		Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: value},
		Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: value},
	}, &updateResp)

	if updateResp.Diagnostics.HasError() {
		test.Fatalf("update: %v", updateResp.Diagnostics)
	}

	var want []any = []any{
		map[string]any{"resource": map[string]any{"table": "users", "priority": 2, "_meta": map[string]any{"path": []any{"users"}}}, "target": "acmeCo/users"},
		map[string]any{"resource": map[string]any{"table": "orders"}, "target": "acmeCo/orders"},
	}

	if !reflect.DeepEqual(backend.published["bindings"], want) {
		test.Errorf("published bindings = %v, want the live resource path kept on users only", backend.published["bindings"])
	}
}

func TestKeepUnmanagedBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var discovering CaptureSpec[testConfig, testResource] = testCaptureSpec
	var plain CaptureSpec[testConfig, testResource] = testCaptureSpec
	var backend *testBackend = &testBackend{live: liveCapture(map[string]any{"target": "acmeCo/events"})}
	var readLive liveReader

	discovering.AutoDiscover = &capture.AutoDiscover{AddNewBindings: true}
	plain.Bindings = []capture.Binding[testResource]{{Resource: testResource{Table: "orders"}, Target: "acmeCo/orders"}}
	discovering.Bindings = plain.Bindings

	entry := newTestCaptureEntry(backend)

	kept, err := entry.codec.Value("movies", plain)

	if err != nil {
		test.Fatal(err)
	}

	managed, err := withValue(kept, keepUnmanagedBindingsPath, false)

	if err != nil {
		test.Fatal(err)
	}

	discovered, err := entry.codec.Value("movies", discovering)

	if err == nil {
		discovered, err = withValue(discovered, keepUnmanagedBindingsPath, false)
	}

	if err != nil {
		test.Fatal(err)
	}

	var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

	entry.Update(ctx, resource.UpdateRequest{Plan: tfsdk.Plan{Schema: entry.codec.Schema(), Raw: kept}, State: tfsdk.State{Schema: entry.codec.Schema(), Raw: kept}}, &updateResp)

	if published, _ := backend.published["bindings"].([]any); len(published) != 2 {
		test.Errorf("published bindings = %v, want the undeclared live binding kept by default", backend.published["bindings"])
	}

	if diagnostics := entry.validateKeepUnmanagedBindings(discovered); !diagnostics.HasError() ||
		!strings.Contains(diagnostics[0].Summary(), "auto_discover") {
		test.Errorf("diagnostics = %v, want false rejected with auto_discover", diagnostics)
	}

	if diagnostics := entry.validateKeepUnmanagedBindings(managed); diagnostics.HasError() {
		test.Errorf("diagnostics = %v, want false accepted without auto_discover", diagnostics)
	}

	readLive = entry.newLiveReader(ctx, kept)

	warnings := entry.warnUnmanagedBindingsRemoval(managed, kept, readLive)

	if len(warnings) != 1 || !strings.Contains(warnings[0].Detail(), "- acmeCo/events\n") || strings.Contains(warnings[0].Detail(), "acmeCo/orders") {
		test.Errorf("warnings = %v, want only the undeclared events binding listed", warnings)
	}

	if warnings := entry.warnUnmanagedBindingsRemoval(managed, managed, readLive); len(warnings) != 0 {
		test.Errorf("warnings = %v, want none once state already manages every binding", warnings)
	}
}
