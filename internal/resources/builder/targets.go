package builder

import (
	"context"
	"fmt"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// planTargets fills each unset binding target from state, else live, else discovery, so the provider never replicates
// connector naming rules. Targets stay unknown while the resources aren't known.
func (entryResource *entryResource[D, S]) planTargets(ctx context.Context, plan tftypes.Value, config tftypes.Value, state tftypes.Value, readLive liveReader) (tftypes.Value, error) {
	var unset map[int]bool = map[int]bool{}
	var resolved map[int]string

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

	if paths, isKnown := entryResource.resourcePaths(plan); isKnown {
		resolved, err = entryResource.resolveTargets(ctx, unset, paths, plan, config, state, readLive)
	}

	if err != nil {
		return plan, err
	}

	return tftypes.Transform(plan, func(path *tftypes.AttributePath, current tftypes.Value) (tftypes.Value, error) {
		index, isTarget := bindingAttributeIndex(path, bindingTargetAttribute)

		if !isTarget || !unset[index] {
			return current, nil
		}

		if target, found := resolved[index]; found {
			return tftypes.NewValue(tftypes.String, target), nil
		}

		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue), nil
	})
}

// resolveTargets returns the unset binding targets by index. Discovery runs only when state and live miss some and
// config is known, failing the plan for resources it doesn't find.
func (entryResource *entryResource[D, S]) resolveTargets(ctx context.Context, unset map[int]bool, paths []string, plan tftypes.Value, config tftypes.Value, state tftypes.Value, readLive liveReader) (map[int]string, error) {
	var resolved map[int]string = map[int]string{}
	var targets map[string]string = entryResource.pathTargets(state)

	if missingTargets(unset, paths, targets) != nil {
		if live, found := readLive(); found {
			addTargets(targets, entryResource.pathTargets(live))
		}
	}

	backend, unconfigured := entryResource.backend()

	if missing := missingTargets(unset, paths, targets); missing != nil && config.IsFullyKnown() && unconfigured == nil {
		discovered, err := entryResource.discoverTargets(ctx, backend, plan, config)

		if err != nil {
			return nil, err
		}

		addTargets(targets, discovered)

		if missing = missingTargets(unset, paths, targets); missing != nil {
			var named []string

			for _, index := range missing {
				named = append(named, fmt.Sprintf("bindings[%d] (%s)", index, paths[index]))
			}

			return nil, fmt.Errorf("discovery found no resource for %s. Check that the resources exist and that the capture's user can read them, or set each binding's target", strings.Join(named, ", "))
		}
	}

	for index := range unset {
		if target, found := targets[paths[index]]; found {
			resolved[index] = target
		}
	}

	return resolved, nil
}

// discoverTargets returns discovered targets by resource path, discovering without bindings or autoDiscover so every
// resource is returned.
func (entryResource *entryResource[D, S]) discoverTargets(ctx context.Context, backend catalog.Backend, plan tftypes.Value, config tftypes.Value) (map[string]string, error) {
	var targets map[string]string = map[string]string{}

	value, err := entryResource.codec.secrets.withConfigSecrets(plan, config)

	if err == nil {
		value, err = withoutTargets(value)
	}

	if err != nil {
		return nil, err
	}

	name, spec, err := entryResource.codec.Spec(value)

	if err != nil {
		return nil, err
	}

	identifier, isIdentifier := any(spec).(catalog.BindingsIdentifier)

	if !isIdentifier {
		return targets, nil
	}

	document, err := utils.YAMLDocument(spec)

	if err != nil {
		return nil, err
	}

	ctx = catalog.WithSecrets(ctx, entryResource.codec.secrets.values(value)...)

	discovered, err := backend.DiscoverCapture(ctx, entryResource.catalogName(name), catalog.RawCapture{catalog.EndpointKey: document[catalog.EndpointKey], catalog.BindingsKey: []any{}})

	if err != nil {
		return nil, fmt.Errorf("discovering the default binding targets: %w", err)
	}

	for target, resource := range discovered {
		if path, err := identifier.ResourcePath(resource); err == nil {
			targets[path] = target
		}
	}

	return targets, nil
}

// missingTargets returns the sorted unset binding indexes whose resource targets lacks, nil when none.
func missingTargets(unset map[int]bool, paths []string, targets map[string]string) []int {
	var missing []int

	for index := range paths {
		if _, found := targets[paths[index]]; unset[index] && !found {
			missing = append(missing, index)
		}
	}

	return missing
}

// addTargets adds the targets of resources not held yet, keeping existing ones.
func addTargets(targets map[string]string, more map[string]string) {
	for path, target := range more {
		if _, stored := targets[path]; !stored {
			targets[path] = target
		}
	}
}

// withoutTargets returns value with null binding targets.
func withoutTargets(value tftypes.Value) (tftypes.Value, error) {
	return nullAttributes(value, func(path *tftypes.AttributePath) bool {
		_, isTarget := bindingAttributeIndex(path, bindingTargetAttribute)

		return isTarget
	})
}

// resourcePaths returns the resource path of each binding, reporting false while the spec holds unknown values.
func (entryResource *entryResource[D, S]) resourcePaths(value tftypes.Value) ([]string, bool) {
	value, err := withoutTargets(value)

	if err != nil {
		return nil, false
	}

	_, spec, err := entryResource.codec.Spec(value)

	identifier, isIdentifier := any(spec).(catalog.BindingsIdentifier)

	if err != nil || !isIdentifier {
		return nil, false
	}

	return identifier.ResourcePaths(), true
}

// pathTargets returns the binding targets by resource path.
func (entryResource *entryResource[D, S]) pathTargets(value tftypes.Value) map[string]string {
	var targets map[string]string = map[string]string{}

	items, _, err := bindings(value)
	paths, isKnown := entryResource.resourcePaths(value)

	if err != nil || !isKnown || len(paths) != len(items) {
		return targets
	}

	for index, binding := range items {
		if target := bindingCollectionValue(binding); target != "" {
			targets[paths[index]] = target
		}
	}

	return targets
}
