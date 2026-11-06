package builder

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// planResourcePaths plans the resource_path of each binding of plan: the state one of the binding with its collection
// and resource, else the one a capture resource declares, else, for materializations, the one Estuary gives it when
// validating the planned bindings, see validatedResourcePaths. They stay unknown on a create and while a binding is.
func (entryResource *entryResource[D, S]) planResourcePaths(ctx context.Context, plan tftypes.Value, state tftypes.Value) (tftypes.Value, error) {
	var declared func(binding tftypes.Value) ([]string, bool) = entryResource.catalogType.resourcePath.declared
	var isCapture bool = entryResource.catalogType.name == "capture"
	var validate bool

	planned, _, err := bindings(plan)

	if err != nil {
		return plan, err
	}

	stored, _, err := bindings(state)

	if err != nil {
		return plan, err
	}

	var paths []tftypes.Value = make([]tftypes.Value, len(planned))

	for index, binding := range planned {
		paths[index] = tftypes.NewValue(resourcePathType, tftypes.UnknownValue)

		if path, found := storedResourcePath(binding, stored); found {
			paths[index] = resourcePathValue(path)
		} else if !isCapture {
			validate = true
		} else if declared == nil {
			paths[index] = tftypes.NewValue(resourcePathType, nil)
		} else if path, isKnown := declared(binding); isKnown {
			paths[index] = resourcePathValue(path)
		}
	}

	if validate && exists(state) {
		validated, err := entryResource.validatedResourcePaths(ctx, plan, state)

		if err != nil {
			return plan, err
		}

		if validated != nil {
			paths = validated
		}
	}

	return withResourcePaths(plan, func(index int, _ tftypes.Value) tftypes.Value {
		return paths[index]
	})
}

// storedResourcePath returns the resource path of the state binding with the collection and resource of binding,
// reporting false when none holds one.
func storedResourcePath(binding tftypes.Value, stored []tftypes.Value) ([]string, bool) {
	var resource tftypes.Value = blockAttribute(binding, bindingResourceAttribute, tftypes.DynamicPseudoType)
	var collection string = bindingCollectionValue(binding)

	if collection == "" || !resource.IsFullyKnown() {
		return nil, false
	}

	for _, candidate := range stored {
		if bindingCollectionValue(candidate) == collection && blockAttribute(candidate, bindingResourceAttribute, tftypes.DynamicPseudoType).Equal(resource) {
			return bindingResourcePath(candidate)
		}
	}

	return nil, false
}

// validatedResourcePaths returns the resource path Estuary gives each planned binding of a live materialization by
// validating them in a dry run, nil while the plan or live spec can't be read. The dry run publishes the planned spec
// with the sealed live endpoint, which plans never hold, and the live counter of each binding holding a live resource,
// see holdsResource, as Estuary rejects lowered counters. Bindings Terraform doesn't manage are left out.
func (entryResource *entryResource[D, S]) validatedResourcePaths(ctx context.Context, plan tftypes.Value, state tftypes.Value) ([]tftypes.Value, error) {
	var paths []tftypes.Value

	name, isKnown := known[string](blockAttribute(state, nameAttribute, tftypes.String))

	if !isKnown || !plan.IsFullyKnown() {
		return nil, nil
	}

	var catalogName string = entryResource.storedCatalogName(state, name)

	document, err := entryResource.codec.Document(plan)

	if err != nil {
		return nil, nil
	}

	sealed, err := entryResource.readSealed(ctx, catalogName)

	if err != nil || sealed == nil {
		return nil, nil
	}

	backend, err := entryResource.backend()

	if err != nil {
		return nil, err
	}

	document[catalog.EndpointKey] = sealed[catalog.EndpointKey]

	delete(document, catalog.ExpectPubIDKey)

	for _, binding := range catalog.Bindings(document) {
		object, _ := binding.(map[string]any)
		live, _ := holdingBinding(binding, catalog.Bindings(sealed)).(map[string]any)

		if counter, found := live[catalog.BackfillKey]; found && object != nil {
			object[catalog.BackfillKey] = counter
		}

		catalog.KeepResourceMeta(live, binding)
	}

	validated, err := backend.ResourcePaths(ctx, catalogName, catalog.RawMaterialization(document))

	if err != nil {
		return nil, fmt.Errorf("validating the planned bindings of %s to read their resource paths: %w", catalogName, err)
	}

	if len(validated) != len(catalog.Bindings(document)) {
		return nil, fmt.Errorf("validating the planned bindings of %s returned %d resource paths for %d bindings", catalogName, len(validated), len(catalog.Bindings(document)))
	}

	for _, path := range validated {
		paths = append(paths, resourcePathValue(path))
	}

	return paths, nil
}

// holdingBinding returns the only live binding document of the collection of binding whose resource holds every field
// of its resource, see holdsResource, nil when none or several do.
func holdingBinding(binding any, live []any) any {
	var holding any

	for _, candidate := range live {
		if catalog.BindingCollection(candidate) != catalog.BindingCollection(binding) || !holdsResource(candidate, binding) {
			continue
		}

		if holding != nil {
			return nil
		}

		holding = candidate
	}

	return holding
}

// holdsResource reports whether the resource of the live binding document sets every field of the resource of the
// planned one, metadata aside, to the same value. Live resources also hold the defaults Estuary fills in.
func holdsResource(live any, planned any) bool {
	var liveResource map[string]any = catalog.BindingResource(live)
	var plannedResource map[string]any = maps.Clone(catalog.BindingResource(planned))

	delete(plannedResource, catalog.ResourceMetaKey)

	if len(plannedResource) == 0 {
		return false
	}

	for key, value := range plannedResource {
		if !reflect.DeepEqual(liveResource[key], value) {
			return false
		}
	}

	return true
}

// publishedResourcePaths returns stored with the resource paths the plan couldn't know, see planResourcePaths, set to
// those Estuary recorded for the published bindings, read through holdingBinding. A path it can't read is null, for
// the next refresh to fill in.
func (entryResource *entryResource[D, S]) publishedResourcePaths(ctx context.Context, catalogName string, stored tftypes.Value) (tftypes.Value, error) {
	var unknown bool
	var live []any
	var published []any

	planned, _, err := bindings(stored)

	if err != nil {
		return stored, err
	}

	for _, binding := range planned {
		unknown = unknown || !blockAttribute(binding, resourcePathAttribute, resourcePathType).IsKnown()
	}

	if !unknown {
		return stored, nil
	}

	if document, err := entryResource.codec.Document(stored); err == nil {
		published = catalog.Bindings(document)
	}

	if sealed, err := entryResource.readSealed(ctx, catalogName); err == nil {
		live = catalog.Bindings(sealed)
	}

	return withResourcePaths(stored, func(index int, binding tftypes.Value) tftypes.Value {
		var current tftypes.Value = blockAttribute(binding, resourcePathAttribute, resourcePathType)

		if current.IsKnown() {
			return current
		}

		if index < len(published) {
			return resourcePathValue(catalog.BindingResourcePath(holdingBinding(published[index], live)))
		}

		return tftypes.NewValue(resourcePathType, nil)
	})
}

// resourcePathValue returns the resource_path value of a resource path, null without one.
func resourcePathValue(path []string) tftypes.Value {
	var components []tftypes.Value

	if len(path) == 0 {
		return tftypes.NewValue(resourcePathType, nil)
	}

	for _, component := range path {
		components = append(components, tftypes.NewValue(tftypes.String, component))
	}

	return tftypes.NewValue(resourcePathType, components)
}

// bindingResourcePath returns the resource_path of a binding value, reporting false while it's null or unknown.
func bindingResourcePath(binding tftypes.Value) ([]string, bool) {
	var components []tftypes.Value
	var path []string

	value := blockAttribute(binding, resourcePathAttribute, resourcePathType)

	if !value.IsKnown() || value.IsNull() || value.As(&components) != nil || len(components) == 0 {
		return nil, false
	}

	for _, component := range components {
		text, isText := known[string](component)

		if !isText {
			return nil, false
		}

		path = append(path, text)
	}

	return path, true
}

// resourcePathKey joins a resource path with NULL, which resource names can't hold, empty without one.
func resourcePathKey(path []string) string {
	return strings.Join(path, "\x00")
}

// withResourcePaths returns value with the resource_path of each binding replaced by what replace returns for it, given
// the binding index and value.
func withResourcePaths(value tftypes.Value, replace func(index int, binding tftypes.Value) tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(value, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		var steps []tftypes.AttributePathStep = path.Steps()
		var attributes map[string]tftypes.Value

		if len(steps) != 2 || steps[0] != tftypes.AttributeName(bindingsAttribute) || !current.IsKnown() || current.IsNull() {
			return current, nil
		}

		index, isIndex := steps[1].(tftypes.ElementKeyInt)

		if !isIndex {
			return current, nil
		}

		if err := current.As(&attributes); err != nil {
			return current, err
		}

		if _, found := attributes[resourcePathAttribute]; !found {
			return current, nil
		}

		attributes[resourcePathAttribute] = replace(int(index), current)

		return tftypes.NewValue(current.Type(), attributes), nil
	})
}
