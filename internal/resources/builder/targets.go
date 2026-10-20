package builder

import (
	"context"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// liveReader returns the live value of the entry in state, read once and found being false when it can't be read.
type liveReader func() (tftypes.Value, bool)

// newLiveReader returns the liveReader of the entry in state, which never reads an entry not created yet.
func (entryResource *entryResource[D, S]) newLiveReader(ctx context.Context, state tftypes.Value) liveReader {
	var live tftypes.Value
	var found, read bool

	return func() (tftypes.Value, bool) {
		if read {
			return live, found
		}

		read = true

		name, err := attributeValue(state, tftypes.NewAttributePath().WithAttributeName("name"))

		var text string

		if err != nil || name.As(&text) != nil || entryResource.session == nil || entryResource.session.Backend == nil {
			return live, found
		}

		sealed, err := entryResource.session.Backend.ReadSpec(ctx, entryResource.catalogName(text))

		if err != nil || sealed == nil {
			return live, found
		}

		live, err = entryResource.liveValue(text, sealed)
		found = err == nil

		return live, found
	}
}

// planTargets returns plan with the target of each binding config leaves unset: the state target of the same resource,
// else the live one, else the collection discovery names, the capture name without its connector name followed by the
// resource recommended name. Bindings are the same resource when their recommended names are. Targets stay unknown
// until apply while the names aren't known, rather than keeping the target Terraform proposes from another binding.
func (entryResource *entryResource[D, S]) planTargets(plan tftypes.Value, config tftypes.Value, state tftypes.Value, readLive liveReader) (tftypes.Value, error) {
	var unset map[int]bool = map[int]bool{}
	var text string
	var liveAdded bool

	configBindings, _, err := bindings(config)

	if err != nil {
		return plan, err
	}

	for index, binding := range configBindings {
		if target := bindingAttribute(binding, bindingTargetAttribute); target.IsKnown() && target.IsNull() {
			unset[index] = true
		}
	}

	if len(unset) == 0 {
		return plan, nil
	}

	name, err := attributeValue(plan, tftypes.NewAttributePath().WithAttributeName("name"))
	names, known := entryResource.recommendedNames(plan)
	known = known && err == nil && name.IsKnown() && name.As(&text) == nil

	var targets map[string]string = entryResource.recommendedTargets(state)
	var prefix string = catalog.Name(entryResource.catalogName(text), entryResource.connector.ConnectorName) + "/"

	return tftypes.Transform(plan, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		index, isTarget := bindingAttributeIndex(path, bindingTargetAttribute)

		if !isTarget || !unset[index] {
			return current, nil
		}

		if !known || names[index] == "" {
			return tftypes.NewValue(tftypes.String, tftypes.UnknownValue), nil
		}

		if _, found := targets[names[index]]; !found && !liveAdded {
			entryResource.addLiveTargets(targets, readLive)
			liveAdded = true
		}

		if target, found := targets[names[index]]; found {
			return tftypes.NewValue(tftypes.String, target), nil
		}

		return tftypes.NewValue(tftypes.String, prefix+names[index]), nil
	})
}

// addLiveTargets adds the live targets of the resources targets doesn't hold yet, the state ones taking precedence.
func (entryResource *entryResource[D, S]) addLiveTargets(targets map[string]string, readLive liveReader) {
	live, found := readLive()

	if !found {
		return
	}

	for name, target := range entryResource.recommendedTargets(live) {
		if _, stored := targets[name]; !stored {
			targets[name] = target
		}
	}
}

// recommendedNames returns the recommended name of each binding of value, known being false when the spec can't be
// decoded yet, such as when the plan holds unknown values.
func (entryResource *entryResource[D, S]) recommendedNames(value tftypes.Value) ([]string, bool) {
	value, err := nullAttributes(value, func(path *tftypes.AttributePath) bool {
		_, isTarget := bindingAttributeIndex(path, bindingTargetAttribute)

		return isTarget
	})

	if err != nil {
		return nil, false
	}

	_, spec, err := entryResource.codec.Spec(value)

	namer, isNamer := any(spec).(catalog.BindingsNamer)

	if err != nil || !isNamer {
		return nil, false
	}

	return namer.RecommendedNames(), true
}

// recommendedTargets returns the targets of the bindings of value by their recommended name.
func (entryResource *entryResource[D, S]) recommendedTargets(value tftypes.Value) map[string]string {
	var targets map[string]string = map[string]string{}

	items, _, err := bindings(value)
	names, known := entryResource.recommendedNames(value)

	if err != nil || !known || len(names) != len(items) {
		return targets
	}

	for index, binding := range items {
		if target := bindingTarget(binding); target != "" && names[index] != "" {
			targets[names[index]] = target
		}
	}

	return targets
}

// bindingAttributeIndex returns the binding index of a bindings[index].attribute path, found being false for other paths.
func bindingAttributeIndex(path *tftypes.AttributePath, attribute string) (int, bool) {
	var steps []tftypes.AttributePathStep = path.Steps()

	if len(steps) != 3 || steps[0] != tftypes.AttributeName(bindingsAttribute) || steps[2] != tftypes.AttributeName(attribute) {
		return 0, false
	}

	index, isIndex := steps[1].(tftypes.ElementKeyInt)

	return int(index), isIndex
}
