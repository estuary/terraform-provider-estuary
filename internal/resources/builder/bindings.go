package builder

import (
	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
)

const resourceMetaKey string = "_meta"

// mergeBindings publishes the planned bindings over the live ones, keeping the live bindings Terraform never
// managed and dropping the previously managed ones no longer planned. New planned bindings follow, in plan order.
func mergeBindings(planned []any, previous []any, live []any) []any {
	var pending map[string]any = map[string]any{}
	var managed map[string]bool = map[string]bool{}

	var merged []any

	for _, binding := range planned {
		pending[bindingCollection(binding)] = binding
	}

	for _, binding := range previous {
		managed[bindingCollection(binding)] = true
	}

	for _, binding := range live {
		var collection string = bindingCollection(binding)

		if plannedBinding, found := pending[collection]; found {
			merged = append(merged, plannedBinding)
			delete(pending, collection)
		} else if !managed[collection] {
			merged = append(merged, binding)
		}
	}

	for _, binding := range planned {
		if _, found := pending[bindingCollection(binding)]; found {
			merged = append(merged, binding)
		}
	}

	return merged
}

// liveDocument returns a sealed live spec as the provider manages it: without its encrypted secrets, which can't be
// decrypted, and without the metadata the control plane adds to each binding resource, which users don't write.
func liveDocument(sealed map[string]any) map[string]any {
	var live map[string]any = catalog.WithoutSecrets(sealed)

	liveBindings, _ := live["bindings"].([]any)

	for _, binding := range liveBindings {
		delete(utils.Lookup(binding, "resource"), resourceMetaKey)
	}

	return live
}

// generatesBindings reports whether the control plane may add bindings to the spec, such as discovered ones.
func generatesBindings(spec any) bool {
	generator, isGenerator := spec.(catalog.BindingsGenerator)

	return isGenerator && generator.GeneratesBindings()
}

// bindingCollection returns the collection of a binding: the target of a capture binding, or the source of a
// materialization binding, either a collection name or an object naming it.
func bindingCollection(binding any) string {
	object, _ := binding.(map[string]any)

	if target, isName := object["target"].(string); isName {
		return target
	}

	if source, isName := object["source"].(string); isName {
		return source
	}

	name, _ := utils.Lookup(object, "source")["name"].(string)

	return name
}
