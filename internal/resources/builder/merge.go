package builder

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v3"
)

// mergeLive merges the plan over the live spec, keeping live settings Terraform doesn't manage, unmanaged bindings,
// resource metadata, backfill counters and the encrypted endpoint when unchanged. It returns the resets and a
// copy of the live spec, nil when the entry doesn't exist.
func (entryResource *entryResource[D, S]) mergeLive(ctx context.Context, catalogName string, document map[string]any, generated bool, plan tftypes.Value, prior tftypes.Value) ([]string, map[string]any, error) {
	live, err := entryResource.session.Backend.ReadSpec(ctx, catalogName)

	if err != nil || live == nil {
		return nil, nil, err
	}

	original, err := utils.YAMLDocument(live)

	if err != nil {
		return nil, nil, err
	}

	priorDocument, err := entryResource.codec.Document(prior)

	if err != nil {
		return nil, nil, err
	}

	for key, value := range live {
		if _, planned := document[key]; !planned && priorDocument[key] == nil {
			document[key] = value
		}
	}

	current, _ := live[bindingsAttribute].([]any)

	if generated {
		planned, _ := document[bindingsAttribute].([]any)
		previous, _ := priorDocument[bindingsAttribute].([]any)

		document[bindingsAttribute] = mergeBindings(planned, previous, current)
	}

	keepResourceMetadata(document[bindingsAttribute], current)

	var wide []string = captureWide(document[bindingsAttribute], current)

	backfills, resets, err := backfillRequests(plan, prior, wide)

	if err != nil {
		return nil, nil, err
	}

	if holder, isHolder := any(*new(D)).(backfillModeHolder); isHolder {
		missing, err := unbackfilledModes(document[bindingsAttribute], current, holder.resourceBackfillMode(), plan, prior, wide)

		if err != nil {
			return nil, nil, err
		}

		var details []string

		for _, collection := range slices.Sorted(maps.Keys(missing)) {
			details = append(details, modeBackfillDetail(collection, missing[collection]))
		}

		if len(details) > 0 {
			return nil, nil, errors.New(strings.Join(details, "\n"))
		}

		for collection := range modeBackfills(document[bindingsAttribute], current, holder.resourceBackfillMode()) {
			if !slices.Contains(backfills, collection) {
				backfills = append(backfills, collection)
			}
		}
	}

	keepBackfills(document[bindingsAttribute], current, backfills)

	if entryResource.codec.secrets.changed(plan, prior) {
		tflog.Debug(ctx, "Publishing the planned endpoint, its secrets changed", map[string]any{"name": catalogName})

		return resets, original, nil
	}

	plannedDocument, err := entryResource.codec.Document(plan)

	if err != nil {
		return nil, nil, err
	}

	if !reflect.DeepEqual(plannedDocument["endpoint"], priorDocument["endpoint"]) {
		tflog.Debug(ctx, "Publishing the planned endpoint, it changed", map[string]any{"name": catalogName, "planned": endpointYAML(plannedDocument), "prior": endpointYAML(priorDocument)})

		return resets, original, nil
	}

	tflog.Debug(ctx, "Keeping the sealed live endpoint", map[string]any{"name": catalogName})

	document["endpoint"] = live["endpoint"]

	return resets, original, nil
}

// endpointYAML returns a document's endpoint as YAML for logs, safe since plans never hold write-only values.
func endpointYAML(document map[string]any) string {
	content, err := yaml.Marshal(document["endpoint"])

	if err != nil {
		return err.Error()
	}

	return string(content)
}
