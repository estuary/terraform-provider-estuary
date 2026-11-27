package builder

import (
	"context"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder/translator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"gopkg.in/yaml.v3"
)

// connectorEntry is the resource of a connector under the acmeCo/ tenant, with the in-memory backend holding its live
// spec.
type connectorEntry[D Document[D, S], S any] struct {
	*entryResource[D, S]
	backend *testBackend
}

// newConnectorCapture returns the capture resource of connector over the sealed live spec, a YAML document.
func newConnectorCapture[C, R any](test *testing.T, connector Connector, live string) connectorEntry[captureDocument[C, R], CaptureSpec[C, R]] {
	test.Helper()

	return configuredConnector(test, NewCapture[C, R](connector)().(*entryResource[captureDocument[C, R], CaptureSpec[C, R]]), live)
}

// newConnectorMaterialization returns the materialization resource of connector over the sealed live spec, a YAML
// document.
func newConnectorMaterialization[C, R any](test *testing.T, connector Connector, live string) connectorEntry[materializationDocument[C, R], MaterializationSpec[C, R]] {
	test.Helper()

	return configuredConnector(test, NewMaterialization[C, R](connector)().(*entryResource[materializationDocument[C, R], MaterializationSpec[C, R]]), live)
}

func configuredConnector[D Document[D, S], S any](test *testing.T, entry *entryResource[D, S], live string) connectorEntry[D, S] {
	test.Helper()

	var configureResp resource.ConfigureResponse
	var sealed map[string]any

	if err := yaml.Unmarshal(spectest.Dedent([]byte(live)), &sealed); err != nil {
		test.Fatalf("decoding the live spec: %v", err)
	}

	var backend *testBackend = &testBackend{live: sealed}

	entry.Configure(context.Background(), resource.ConfigureRequest{ProviderData: &catalog.Session{Backend: backend, Tenant: "acmeCo/"}}, &configureResp)

	if configureResp.Diagnostics.HasError() {
		test.Fatalf("configure: %v", configureResp.Diagnostics)
	}

	return connectorEntry[D, S]{entryResource: entry, backend: backend}
}

// importState returns the state terraform import leaves for id: the imported one, refreshed. WarnsUnmanaged is
// whether the import must warn about live fields the provider doesn't manage.
func (entry connectorEntry[D, S]) importState(test *testing.T, id string, warnsUnmanaged bool) tftypes.Value {
	test.Helper()

	var ctx context.Context = context.Background()
	var importResp resource.ImportStateResponse = resource.ImportStateResponse{
		State: tfsdk.State{Schema: entry.codec.Schema(), Raw: tftypes.NewValue(entry.codec.valueType, nil)},
	}

	entry.ImportState(ctx, resource.ImportStateRequest{ID: id}, &importResp)

	if importResp.Diagnostics.HasError() {
		test.Fatalf("import: %v", importResp.Diagnostics)
	}

	if warned := importResp.Diagnostics.WarningsCount() > 0; warned != warnsUnmanaged {
		test.Errorf("import warnings = %v, want a warning about unmanaged fields = %v", importResp.Diagnostics.Warnings(), warnsUnmanaged)
	}

	var readResp resource.ReadResponse = resource.ReadResponse{State: importResp.State}

	entry.Read(ctx, resource.ReadRequest{State: importResp.State}, &readResp)

	if readResp.Diagnostics.HasError() || readResp.Diagnostics.WarningsCount() > 0 {
		test.Fatalf("refresh after the import: %v", readResp.Diagnostics)
	}

	return readResp.State.Raw
}

// proposedPlan returns the plan Terraform proposes for config over state: config without its write-only secrets, and
// with the state value of each computed attribute it leaves null, unknown instead when the plan changes anything.
func (entry connectorEntry[D, S]) proposedPlan(test *testing.T, config tftypes.Value, state tftypes.Value) tftypes.Value {
	test.Helper()

	var computedAndUnset func(path *tftypes.AttributePath) bool = func(path *tftypes.AttributePath) bool {
		attribute, err := entry.codec.secrets.schema.AttributeAtTerraformPath(context.Background(), path)

		if err != nil || !attribute.IsComputed() {
			return false
		}

		configured, err := attributeValue(config, path)

		return err != nil || configured.IsNull()
	}

	withoutSecrets, err := entry.codec.secrets.nullSecrets(config)

	if err != nil {
		test.Fatal(err)
	}

	proposed, err := tftypes.Transform(withoutSecrets, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		if !computedAndUnset(path) {
			return current, nil
		}

		if stored, err := attributeValue(state, path); err == nil {
			return stored, nil
		}

		return tftypes.NewValue(current.Type(), nil), nil
	})

	if err != nil {
		test.Fatal(err)
	}

	if proposed.Equal(state) {
		return proposed
	}

	proposed, err = tftypes.Transform(proposed, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		if !computedAndUnset(path) {
			return current, nil
		}

		return tftypes.NewValue(current.Type(), tftypes.UnknownValue), nil
	})

	if err != nil {
		test.Fatal(err)
	}

	return proposed
}

// plan plans the configured spec named name over state, as terraform plan does, returning the configuration and the
// plan.
func (entry connectorEntry[D, S]) plan(test *testing.T, name string, configured S, state tftypes.Value) (tftypes.Value, resource.ModifyPlanResponse) {
	test.Helper()

	config, err := entry.codec.Value(name, configured)

	if err == nil {
		config, err = nullAttributes(config, func(path *tftypes.AttributePath) bool {
			_, isName := bindingAttributeElement(path, bindingNameAttribute)

			return isName
		})
	}

	if err != nil {
		test.Fatal(err)
	}

	var modifyPlanReq resource.ModifyPlanRequest = resource.ModifyPlanRequest{
		State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: state},
		Plan:   tfsdk.Plan{Schema: entry.codec.Schema(), Raw: entry.proposedPlan(test, config, state)},
		Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: config},
	}
	var modifyPlanResp resource.ModifyPlanResponse = resource.ModifyPlanResponse{Plan: modifyPlanReq.Plan}

	entry.ModifyPlan(context.Background(), modifyPlanReq, &modifyPlanResp)

	if modifyPlanResp.Diagnostics.HasError() {
		test.Fatalf("plan: %v", modifyPlanResp.Diagnostics)
	}

	if len(modifyPlanResp.RequiresReplace) > 0 {
		test.Errorf("plan replaces %v, want the imported entry kept", modifyPlanResp.RequiresReplace)
	}

	return config, modifyPlanResp
}

// assertPlansNoChanges checks that the configured spec plans exactly the imported state, without a warning, so
// terraform plan reports no changes.
func (entry connectorEntry[D, S]) assertPlansNoChanges(test *testing.T, name string, configured S, state tftypes.Value) {
	test.Helper()

	_, modifyPlanResp := entry.plan(test, name, configured, state)

	if warnings := modifyPlanResp.Diagnostics.Warnings(); len(warnings) > 0 {
		test.Errorf("plan warnings = %v, want none", warnings)
	}

	if !modifyPlanResp.Plan.Raw.Equal(state) {
		test.Errorf("plan differs from the imported state, want no changes:\n%s", valueDifferences(modifyPlanResp.Plan.Raw, state))
	}
}

// apply applies the configured spec named name over state, as terraform apply does, returning the spec it published,
// nil when it left the live one as it is, and the plan warnings.
func (entry connectorEntry[D, S]) apply(test *testing.T, name string, configured S, state tftypes.Value) (map[string]any, diag.Diagnostics) {
	test.Helper()

	config, modifyPlanResp := entry.plan(test, name, configured, state)

	var updateResp resource.UpdateResponse = resource.UpdateResponse{State: tfsdk.State{Schema: entry.codec.Schema()}}

	entry.backend.published = nil

	entry.Update(context.Background(), resource.UpdateRequest{
		State:  tfsdk.State{Schema: entry.codec.Schema(), Raw: state},
		Plan:   modifyPlanResp.Plan,
		Config: tfsdk.Config{Schema: entry.codec.Schema(), Raw: config},
	}, &updateResp)

	if updateResp.Diagnostics.HasError() {
		test.Fatalf("apply: %v", updateResp.Diagnostics)
	}

	return entry.backend.published, modifyPlanResp.Diagnostics.Warnings()
}

// assertAppliesWithoutPublishing checks that applying the configured spec over the imported state leaves the live
// spec as it is, publishing nothing. Unlike assertPlansNoChanges, the spec may declare bindings, which the apply takes
// over from the live ones.
func (entry connectorEntry[D, S]) assertAppliesWithoutPublishing(test *testing.T, name string, configured S, state tftypes.Value) {
	test.Helper()

	if published, _ := entry.apply(test, name, configured, state); published != nil {
		content, _ := yaml.Marshal(published)

		test.Errorf("the apply published a spec, want the live one left as it is:\n%s", content)
	}
}

// valueDifferences lists the attributes whose value differs between got and want, one per line.
func valueDifferences(got tftypes.Value, want tftypes.Value) string {
	differences, err := got.Diff(want)

	if err != nil {
		return err.Error()
	}

	var lines string

	for _, difference := range differences {
		lines += difference.String() + "\n"
	}

	return lines
}

// configAttributePath returns the path of the config attribute under the nested attribute names.
func configAttributePath(names ...string) *tftypes.AttributePath {
	var path *tftypes.AttributePath = configPath

	for _, name := range names {
		path = path.WithAttributeName(name)
	}

	return path
}

// assertWriteOnly checks that the secrets at paths, which config must set, are exactly its write-only values: the
// plan and the state hold none of them, and the state holds a salted hash beside each.
func assertWriteOnly(test *testing.T, secrets secrets, config tftypes.Value, paths ...*tftypes.AttributePath) {
	test.Helper()

	plan, err := secrets.nullSecrets(config)

	if err != nil {
		test.Fatal(err)
	}

	state, err := secrets.stateHashes(plan, config)

	if err != nil {
		test.Fatal(err)
	}

	if values := secrets.values(config); len(values) != len(paths) {
		test.Errorf("the configuration holds %d write-only values, want the %d secrets listed", len(values), len(paths))
	}

	for _, path := range paths {
		var secret, hash string

		if !secrets.writeOnly(path) {
			test.Errorf("%s isn't write-only", path)
			continue
		}

		if configured, err := attributeValue(config, path); err != nil || configured.As(&secret) != nil || secret == "" {
			test.Errorf("%s isn't set in the configuration, so the test can't tell it is kept out of state", path)
			continue
		}

		for label, value := range map[string]tftypes.Value{"plan": plan, "state": state} {
			if stored, err := attributeValue(value, path); err != nil || !stored.IsNull() {
				test.Errorf("%s of the %s = %v (%v), want null", path, label, stored, err)
			}
		}

		var steps []tftypes.AttributePathStep = path.Steps()
		var name tftypes.AttributeName = steps[len(steps)-1].(tftypes.AttributeName)
		var hashPath *tftypes.AttributePath = tftypes.NewAttributePathWithSteps(steps[:len(steps)-1]).WithAttributeName(string(name) + translator.HashSuffix)

		if stored, err := attributeValue(state, hashPath); err != nil || stored.As(&hash) != nil || !verifySecret(secret, hash) {
			test.Errorf("%s of the state = %v (%v), want the salted hash of the secret", hashPath, stored, err)
		}
	}
}

// assertCaptureSamplesRoundTrip checks each capture sample in directory converts to a Terraform value and back
// unchanged.
func assertCaptureSamplesRoundTrip[C, R any](test *testing.T, connector Connector, directory string) {
	test.Helper()

	codec, err := newCodec[captureDocument[C, R]](connector, captureType[R]())

	if err != nil {
		test.Fatal(err)
	}

	files, err := filepath.Glob(filepath.Join(directory, "*.yaml"))

	if err != nil || len(files) == 0 {
		test.Fatalf("no samples in %s: %v", directory, err)
	}

	for _, file := range files {
		for name, spec := range spectest.DecodeCaptures[CaptureSpec[C, R]](test, file) {
			// The Terraform shape holds backfill triggers, not the counters Estuary keeps.
			for index := range spec.Bindings {
				spec.Bindings[index].Backfill = 0
			}

			value, err := codec.Value(name, spec)

			if err != nil {
				test.Fatalf("%s %s: converting to a Terraform value: %v", file, name, err)
			}

			gotName, got, err := codec.Spec(value)

			if err != nil {
				test.Fatalf("%s %s: converting back to a spec: %v", file, name, err)
			}

			if gotName != name || !reflect.DeepEqual(got, spec) {
				test.Errorf("%s %s: round trip = %s %+v, want %+v", file, name, gotName, got, spec)
			}
		}
	}
}

// assertPlansOnlySecretHashes checks that the configured spec, which sets the secrets at paths, plans the imported
// state plus the hash of each secret, so terraform plan shows those hashes as its only changes. Secrets aren't
// imported, so the first apply publishes the configured ones.
func (entry connectorEntry[D, S]) assertPlansOnlySecretHashes(test *testing.T, name string, configured S, state tftypes.Value, paths ...*tftypes.AttributePath) {
	test.Helper()

	_, modifyPlanResp := entry.plan(test, name, configured, state)

	differences, err := modifyPlanResp.Plan.Raw.Diff(state)

	if err != nil {
		test.Fatal(err)
	}

	var changed []string
	var want []string

	for _, difference := range differences {
		changed = append(changed, difference.Path.String())

		if difference.Value1 == nil || difference.Value1.IsKnown() {
			test.Errorf("planned %s = %v, want it known after apply", difference.Path, difference.Value1)
		}
	}

	for _, path := range paths {
		var steps []tftypes.AttributePathStep = path.Steps()
		var secret tftypes.AttributeName = steps[len(steps)-1].(tftypes.AttributeName)

		want = append(want, tftypes.NewAttributePathWithSteps(steps[:len(steps)-1]).WithAttributeName(string(secret)+translator.HashSuffix).String())
	}

	slices.Sort(changed)
	slices.Sort(want)

	if !slices.Equal(changed, want) {
		test.Errorf("plan changes %v, want only the hashes of the configured secrets, %v", changed, want)
	}
}
