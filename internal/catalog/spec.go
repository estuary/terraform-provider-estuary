package catalog

// RawCapture is a capture spec as YAML, keeping fields the typed specs don't model.
type RawCapture map[string]any

// RawMaterialization is a materialization spec as YAML, keeping fields the typed specs don't model.
type RawMaterialization map[string]any

// Shards holds shard settings (logLevel, disable, ...) free-form, so new settings need no change.
type Shards = map[string]any

// CaptureSpec is implemented by connector capture specs through capture.Spec.
type CaptureSpec interface {
	CaptureSpec()
}

// MaterializationSpec is implemented by connector materialization specs through materialization.Spec.
type MaterializationSpec interface {
	MaterializationSpec()
}

// BindingsGenerator is implemented by specs whose bindings the control plane may add.
type BindingsGenerator interface {
	GeneratesBindings() bool
}

// NewBindingsAdder is implemented by specs whose unbound resources Estuary binds, such as discovered resources or
// source capture collections.
type NewBindingsAdder interface {
	AddsNewBindings() bool
}

// BindingsIdentifier is implemented by specs keying each binding by its resource path.
type BindingsIdentifier interface {
	ResourcePaths() []string
	ResourcePath(resource map[string]any) (string, error)
}

// ConnectorSchema holds a connector's endpoint and resource JSON schemas, kept as test snapshots.
type ConnectorSchema struct {
	Endpoint map[string]any `json:"endpoint"`
	Resource map[string]any `json:"resource"`
}

// CaptureSpec marks RawCapture as a capture spec.
func (RawCapture) CaptureSpec() {}

// MaterializationSpec marks RawMaterialization as a materialization spec.
func (RawMaterialization) MaterializationSpec() {}
