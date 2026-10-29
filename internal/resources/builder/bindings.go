package builder

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// mergeBindings merges planned bindings over live ones, keeping unmanaged ones and dropping removed managed ones.
// New planned bindings follow, in plan order. Collection returns the collection identifying a binding.
func mergeBindings[B any](planned []B, previous []B, live []B, collection func(B) string) []B {
	var pending map[string]B = map[string]B{}
	var managed map[string]bool = map[string]bool{}

	var merged []B

	for _, binding := range planned {
		pending[collection(binding)] = binding
	}

	for _, binding := range previous {
		managed[collection(binding)] = true
	}

	for _, binding := range live {
		var key string = collection(binding)

		if plannedBinding, found := pending[key]; found {
			merged = append(merged, plannedBinding)
			delete(pending, key)
		} else if !managed[key] {
			merged = append(merged, binding)
		}
	}

	for _, binding := range planned {
		if _, found := pending[collection(binding)]; found {
			merged = append(merged, binding)
		}
	}

	return merged
}

// mergedBindings returns the binding documents publishing planned bindings over live ones writes, merged with the
// unmanaged live ones when the control plane generates bindings, and their entryWide collections.
func mergedBindings(planned []any, previous []any, live []any, generated bool) ([]any, []string) {
	var published []any = planned

	if generated {
		published = mergeBindings(planned, previous, live, catalog.BindingCollection)
	}

	return published, entryWide(published, live)
}

// keepResourceMetadata copies control-plane resource metadata, such as the path, from live into planned bindings.
func keepResourceMetadata(planned []any, live []any) {
	var liveBindings map[string]map[string]any = catalog.BindingsByCollection(live)

	for _, binding := range planned {
		catalog.KeepResourceMeta(liveBindings[catalog.BindingCollection(binding)], binding)
	}
}

// generatesBindings reports whether the control plane may add bindings, such as discovered ones.
func generatesBindings(spec any) bool {
	generator, isGenerator := spec.(catalog.BindingsGenerator)

	return isGenerator && generator.GeneratesBindings()
}

// addsNewBindings reports whether Estuary binds every resource the spec doesn't, such as discovered resources.
func addsNewBindings(spec any) bool {
	adder, isAdder := spec.(catalog.NewBindingsAdder)

	return isAdder && adder.AddsNewBindings()
}

// bindings returns an entry's bindings and list type, nil type when null.
func bindings(value tftypes.Value) ([]tftypes.Value, tftypes.Type, error) {
	var attributes map[string]tftypes.Value
	var items []tftypes.Value

	if value.Type() == nil || value.IsNull() || !value.IsKnown() {
		return nil, nil, nil
	}

	if err := value.As(&attributes); err != nil {
		return nil, nil, err
	}

	list, found := attributes[bindingsAttribute]

	if !found || list.IsNull() || !list.IsKnown() {
		return nil, nil, nil
	}

	if err := list.As(&items); err != nil {
		return nil, nil, err
	}

	return items, list.Type(), nil
}

// byCollection indexes binding values by collection.
func byCollection(items []tftypes.Value) map[string]tftypes.Value {
	var indexed map[string]tftypes.Value = map[string]tftypes.Value{}

	for _, binding := range items {
		indexed[bindingCollectionValue(binding)] = binding
	}

	return indexed
}

// indexedBindings returns an entry's binding values by collection.
func indexedBindings(value tftypes.Value) (map[string]tftypes.Value, error) {
	items, _, err := bindings(value)

	return byCollection(items), err
}

// bindingCollectionValue returns a binding value's collection, empty when unknown.
func bindingCollectionValue(binding tftypes.Value) string {
	for _, attribute := range []string{bindingTargetAttribute, bindingSourceAttribute} {
		if collection, isKnown := known[string](bindingAttribute(binding, attribute)); isKnown {
			return collection
		}
	}

	return ""
}

// withBindingNames sets each binding name to its collection, unknown while the collection is.
func withBindingNames(value tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(value, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		var steps []tftypes.AttributePathStep = path.Steps()
		var attributes map[string]tftypes.Value

		if len(steps) != 2 || steps[0] != tftypes.AttributeName(bindingsAttribute) || !current.IsKnown() || current.IsNull() {
			return current, nil
		}

		if err := current.As(&attributes); err != nil {
			return current, err
		}

		collection, isCapture := attributes[bindingTargetAttribute]

		if !isCapture {
			collection = bindingAttribute(current, bindingSourceAttribute)
		}

		attributes[bindingNameAttribute] = collection

		return tftypes.NewValue(current.Type(), attributes), nil
	})
}

// bindingAttribute returns a string attribute of a binding value, unknown when the binding is. Other attributes use
// blockAttribute with their type.
func bindingAttribute(binding tftypes.Value, attribute string) tftypes.Value {
	return blockAttribute(binding, attribute, tftypes.String)
}

// providerOnly reports whether path is applied by the provider but absent from the catalog spec: the entry
// backfill block and binding backfill trigger and strategy.
func providerOnly(path *tftypes.AttributePath) bool {
	return path.Equal(tftypes.NewAttributePath().WithAttributeName(backfillAttribute)) ||
		bindingBackfillPath(path, backfillTriggerAttribute) || bindingBackfillPath(path, backfillStrategyAttribute)
}

// bindingBackfillPath reports whether path is bindings[index].backfill.attribute.
func bindingBackfillPath(path *tftypes.AttributePath, attribute string) bool {
	var steps []tftypes.AttributePathStep = path.Steps()

	if len(steps) != 4 || steps[3] != tftypes.AttributeName(attribute) {
		return false
	}

	_, isBlock := bindingAttributeIndex(tftypes.NewAttributePathWithSteps(steps[:3]), backfillAttribute)

	return isBlock
}

// bindingAttributeIndex returns the index of a bindings[index].attribute path, false for other paths.
func bindingAttributeIndex(path *tftypes.AttributePath, attribute string) (int, bool) {
	var steps []tftypes.AttributePathStep = path.Steps()

	if len(steps) != 3 || steps[0] != tftypes.AttributeName(bindingsAttribute) || steps[2] != tftypes.AttributeName(attribute) {
		return 0, false
	}

	index, isIndex := steps[1].(tftypes.ElementKeyInt)

	return int(index), isIndex
}
