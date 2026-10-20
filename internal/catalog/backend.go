package catalog

import "context"

// RawCapture is a capture spec as a YAML document, publishing fields the typed spec don't model.
type RawCapture map[string]any

// RawMaterialization is a materialization spec as a YAML document, publishing fields the typed specs don't model.
type RawMaterialization map[string]any

// CaptureSpec is implemented by every connector capture spec through capture.Spec.
type CaptureSpec interface {
	CaptureSpec()
}

// MaterializationSpec is implemented by every connector materialization spec through materialization.Spec.
type MaterializationSpec interface {
	MaterializationSpec()
}

// BindingsGenerator is implemented by specs whose bindings the control plane may add, such as discovered ones.
type BindingsGenerator interface {
	GeneratesBindings() bool
}

// NewBindingsAdder is implemented by specs whose discovery adds a binding for each resource not bound, re-adding the removed ones.
type NewBindingsAdder interface {
	AddsNewBindings() bool
}

// BindingsNamer is implemented by specs naming each binding collection after its resource, as discovery does.
type BindingsNamer interface {
	RecommendedNames() []string
}

// Backend is an interface that implements the functions responsible by managing the catalog
// resources providing the contract for the functions required for each backend to perform
// proper serialization and transport for the resource configurations.
type Backend interface {
	PublishCapture(ctx context.Context, name string, spec CaptureSpec) error
	PublishMaterialization(ctx context.Context, name string, spec MaterializationSpec) error
	ReadCatalogEntry(ctx context.Context, name string) (*CatalogEntry, error)
	DeleteCatalogEntry(ctx context.Context, name string) error
	DiscoverCapture(ctx context.Context, name string, spec CaptureSpec) (map[string]DiscoveredStreamInfo, error)
	ConnectorSchema(ctx context.Context, image string) (*ConnectorSchema, error)
	ReadSpec(ctx context.Context, name string) (map[string]any, error)
}

// ConnectorSchema carries the JSON schemas a connector image publishes for its endpoint config
// and binding resources, as rendered by the UI connector forms.
type ConnectorSchema struct {
	Endpoint map[string]any `json:"endpoint"`
	Resource map[string]any `json:"resource"`
}

// CatalogEntry identifies an existing remote catalog resource
type CatalogEntry struct {
	CatalogName string
}

// DiscoveredStreamInfo carries a discovered binding resource, its collection keys and their schemas.
type DiscoveredStreamInfo struct {
	Resource   map[string]any
	Keys       []string
	KeySchemas map[string]any
}

// Session carries the backend and tenant shared by every resource of a configured provider.
type Session struct {
	Backend Backend
	Tenant  string
}

// CaptureSpec marks RawCapture as a capture spec.
func (RawCapture) CaptureSpec() {}

// MaterializationSpec marks RawMaterialization as a materialization spec.
func (RawMaterialization) MaterializationSpec() {}
