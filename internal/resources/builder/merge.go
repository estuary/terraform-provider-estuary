package builder

import (
	"context"
	"reflect"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v3"
)

// mergeLive merges the plan over the live spec, keeping live settings Terraform doesn't manage, unmanaged bindings,
// resource metadata, backfill counters and the encrypted endpoint when unchanged. The live publication ID goes with
// it, so the control plane rejects the publication when another one changed the entry since this read. It returns the
// resets and a copy of the live spec, nil when the entry doesn't exist.
func (entryResource *entryResource[D, S]) mergeLive(ctx context.Context, name string, catalogName string, document map[string]any, keepUnmanaged bool, plan tftypes.Value, prior tftypes.Value) ([]string, map[string]any, error) {
	live, err := entryResource.readSealed(ctx, catalogName)

	if err != nil || live == nil {
		return nil, nil, err
	}

	if pubID, found := live[catalog.ExpectPubIDKey]; found {
		document[catalog.ExpectPubIDKey] = pubID
	}

	original, err := utils.YAMLDocument(live)

	if err != nil {
		return nil, nil, err
	}

	priorDocument, err := entryResource.codec.Document(prior)

	if err != nil {
		return nil, nil, err
	}

	keepUnmanagedKeys(document, priorDocument, live)

	resets, err := entryResource.mergeLiveBindings(name, document, live, keepUnmanaged, plan, prior)

	if err == nil {
		err = entryResource.keepSealedEndpoint(ctx, catalogName, document, priorDocument, live, plan, prior)
	}

	if err != nil {
		return nil, nil, err
	}

	return resets, original, nil
}

// keepUnmanagedKeys copies the live keys the plan doesn't set and Terraform didn't manage.
func keepUnmanagedKeys(document map[string]any, priorDocument map[string]any, live map[string]any) {
	for key, value := range live {
		if _, planned := document[key]; !planned && priorDocument[key] == nil {
			document[key] = value
		}
	}
}

// mergeLiveBindings sets the published bindings, see newPlannedPublication, with the live resource metadata and
// backfill counters, plus one for each backfill. Bindings pair as in the plan, through the Terraform value of the live
// spec. It returns the resets, failing on backfill mode changes and switches from delta to standard updates without a
// backfill.
func (entryResource *entryResource[D, S]) mergeLiveBindings(name string, document map[string]any, live map[string]any, keepUnmanaged bool, plan tftypes.Value, prior tftypes.Value) ([]string, error) {
	var current []any = catalog.Bindings(live)

	liveValue, _, err := entryResource.codec.liveValue(name, live)

	if err != nil {
		return nil, err
	}

	pairing, err := entryResource.catalogType.resourcePath.newPairing(plan, prior, func() (tftypes.Value, bool) { return liveValue, true })

	if err != nil {
		return nil, err
	}

	publication, err := newPlannedPublication(catalog.Bindings(document), current, keepUnmanaged, pairing, entryResource.catalogType)

	if err == nil {
		err = missingBackfillsError(publication.missing)
	}

	if err != nil {
		return nil, err
	}

	keepResourceMetadata(publication.published, current)
	keepBackfills(publication.published, current, publication.backfills)

	if keepUnmanaged {
		document[catalog.BindingsKey] = documents(publication.published)
	}

	return publication.resets, nil
}

// keepSealedEndpoint publishes the sealed live endpoint while the plan changes neither the endpoint nor its secrets,
// so unchanged secrets aren't sent again. A live image other than the planned one, such as one changed outside
// Terraform while refreshes failed, is never kept.
func (entryResource *entryResource[D, S]) keepSealedEndpoint(ctx context.Context, catalogName string, document map[string]any, priorDocument map[string]any, live map[string]any, plan tftypes.Value, prior tftypes.Value) error {
	if entryResource.codec.secrets.changed(plan, prior) {
		tflog.Debug(ctx, "Publishing the planned endpoint, its secrets changed", map[string]any{"name": catalogName})

		return nil
	}

	plannedDocument, err := entryResource.codec.Document(plan)

	if err != nil {
		return err
	}

	if !reflect.DeepEqual(plannedDocument[catalog.EndpointKey], priorDocument[catalog.EndpointKey]) {
		tflog.Debug(ctx, "Publishing the planned endpoint, it changed", map[string]any{"name": catalogName, "planned": endpointYAML(plannedDocument), "prior": endpointYAML(priorDocument)})

		return nil
	}

	if catalog.ConnectorImage(live) != catalog.ConnectorImage(plannedDocument) {
		tflog.Debug(ctx, "Publishing the planned endpoint, the live image differs", map[string]any{"name": catalogName})

		return nil
	}

	tflog.Debug(ctx, "Keeping the sealed live endpoint", map[string]any{"name": catalogName})

	document[catalog.EndpointKey] = live[catalog.EndpointKey]

	return nil
}

// endpointYAML returns a document's endpoint as YAML for logs, safe since plans never hold write-only values.
func endpointYAML(document map[string]any) string {
	content, err := yaml.Marshal(document[catalog.EndpointKey])

	if err != nil {
		return err.Error()
	}

	return string(content)
}
