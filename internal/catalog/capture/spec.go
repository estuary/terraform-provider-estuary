package capture

// AutoDiscover enables the periodic discovery of the capture, and how its results are applied.
type AutoDiscover struct {
	AddNewBindings                bool `yaml:"addNewBindings,omitempty"`
	EvolveIncompatibleCollections bool `yaml:"evolveIncompatibleCollections,omitempty"`
}

// Binding maps a captured resource to the collection it is captured into, R being the connector resource.
type Binding[R any] struct {
	Resource R      `yaml:"resource"`
	Target   string `yaml:"target"`
	Disable  bool   `yaml:"disable,omitempty"`
	Backfill int    `yaml:"backfill,omitempty"`
}

// Spec is a full capture entry keyed by its catalog name under the captures section of a catalog
// spec, E being the endpoint (e.g. endpoint.ConnectorSpec[Config]) and R being the connector resource.
type Spec[E, R any] struct {
	AutoDiscover *AutoDiscover `yaml:"autoDiscover,omitempty"`
	Endpoint     E             `yaml:"endpoint"`
	Bindings     []Binding[R]  `yaml:"bindings"`
}

// NameRecommender is implemented by resources whose collection discovery names after them, e.g. database/collection.
type NameRecommender interface {
	RecommendedName() string
}

// RecommendedNames returns the recommended collection name of each binding, empty without one, satisfying catalog.BindingsNamer.
func (spec Spec[E, R]) RecommendedNames() []string {
	var names []string = make([]string, len(spec.Bindings))

	for index, binding := range spec.Bindings {
		if recommender, isRecommender := any(binding.Resource).(NameRecommender); isRecommender {
			names[index] = recommender.RecommendedName()
		}
	}

	return names
}

// CaptureSpec marks Spec as a capture spec, satisfying catalog.CaptureSpec.
func (Spec[E, R]) CaptureSpec() {}

// AddsNewBindings reports whether auto discovery adds the resources not bound, satisfying catalog.NewBindingsAdder.
func (spec Spec[E, R]) AddsNewBindings() bool {
	return spec.AutoDiscover != nil && spec.AutoDiscover.AddNewBindings
}

// GeneratesBindings reports whether auto discovery may add bindings to the capture, satisfying catalog.BindingsGenerator.
func (spec Spec[E, R]) GeneratesBindings() bool {
	return spec.AutoDiscover != nil
}
