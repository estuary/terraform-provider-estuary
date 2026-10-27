package catalog

import (
	"github.com/estuary/terraform-provider-estuary/internal/utils"
)

// ResourceMetaKey is the binding resource key of control-plane metadata, such as the resource path.
const ResourceMetaKey string = "_meta"

// BindingCollection returns a binding document's collection: the capture target or materialization source.
func BindingCollection(binding any) string {
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
