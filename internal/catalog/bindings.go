package catalog

import (
	"github.com/estuary/terraform-provider-estuary/internal/utils"
)

// Spec document keys of captures and materializations. ResourceMetaKey holds the control-plane metadata of a binding
// resource, such as the resource path. ExpectPubIDKey holds the publication ID the control plane must find live,
// NewEntryPubID asserting no entry has the name yet.
const (
	ExpectPubIDKey  string = "expectPubId"
	NewEntryPubID   string = "00:00:00:00:00:00:00:00"
	EndpointKey     string = "endpoint"
	ConnectorKey    string = "connector"
	ImageKey        string = "image"
	ConfigKey       string = "config"
	BindingsKey     string = "bindings"
	ResourceKey     string = "resource"
	ResourceMetaKey string = "_meta"
	TargetKey       string = "target"
	SourceKey       string = "source"
	SourceNameKey   string = "name"
	BackfillKey     string = "backfill"
	DisableKey      string = "disable"
)

// BindingCollection returns a binding document's collection: the capture target or materialization source.
func BindingCollection(binding any) string {
	object, _ := binding.(map[string]any)

	if target, isName := object[TargetKey].(string); isName {
		return target
	}

	if source, isName := object[SourceKey].(string); isName {
		return source
	}

	name, _ := utils.Lookup(object, SourceKey)[SourceNameKey].(string)

	return name
}

// ConnectorImage returns the connector image of a spec document, empty for other endpoints.
func ConnectorImage(document map[string]any) string {
	image, _ := utils.Lookup(document, EndpointKey, ConnectorKey)[ImageKey].(string)

	return image
}

// Bindings returns the binding documents of a spec document.
func Bindings(document map[string]any) []any {
	bindings, _ := document[BindingsKey].([]any)

	return bindings
}

// BindingDisabled reports whether a binding document sets disable.
func BindingDisabled(binding any) bool {
	object, _ := binding.(map[string]any)
	disable, _ := object[DisableKey].(bool)

	return disable
}

// BindingResource returns a binding document's resource, nil without one.
func BindingResource(binding any) map[string]any {
	return utils.Lookup(binding, ResourceKey)
}

// BindingsByCollection indexes binding documents by collection.
func BindingsByCollection(bindings []any) map[string]map[string]any {
	var indexed map[string]map[string]any = map[string]map[string]any{}

	for _, binding := range bindings {
		object, _ := binding.(map[string]any)

		indexed[BindingCollection(binding)] = object
	}

	return indexed
}

// KeepResourceMeta copies the resource metadata of the from binding document into the to one, unless it has its own.
func KeepResourceMeta(from any, to any) {
	var resource map[string]any = BindingResource(to)

	if meta, found := BindingResource(from)[ResourceMetaKey]; found && resource != nil && resource[ResourceMetaKey] == nil {
		resource[ResourceMetaKey] = meta
	}
}
