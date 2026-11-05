package capture

import (
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
)

// AutoDiscover configures periodic discovery and how its results are applied.
type AutoDiscover struct {
	AddNewBindings                bool `yaml:"addNewBindings,omitempty" description:"Adds a binding for each resource discovery finds in the source that isn't declared, when the capture is created and on each later discovery. Terraform doesn't manage those bindings; declare a binding to manage it."`
	EvolveIncompatibleCollections bool `yaml:"evolveIncompatibleCollections,omitempty" description:"Automatically updates the target collections when the source schema changes in a way they can't accept."`
}

// Binding maps a captured resource to its collection.
type Binding[R any] struct {
	Resource R      `yaml:"resource"`
	Target   string `yaml:"target"`
	Disable  bool   `yaml:"disable,omitempty"`
	Backfill int    `yaml:"backfill,omitempty"`
}

// Spec is a capture entry of a catalog spec, E being the endpoint and R the connector resource.
type Spec[E, R any] struct {
	AutoDiscover *AutoDiscover  `yaml:"autoDiscover,omitempty"`
	Endpoint     E              `yaml:"endpoint"`
	Bindings     []Binding[R]   `yaml:"bindings"`
	Shards       catalog.Shards `yaml:"shards,omitempty"`
}

// ResourcePather is implemented by resources identified by a path in the source, e.g. namespace and stream.
type ResourcePather interface {
	ResourcePath() []string
}

// ResourcePaths returns the resource path key of each binding, satisfying catalog.BindingsIdentifier.
func (spec Spec[E, R]) ResourcePaths() []string {
	var paths []string = make([]string, len(spec.Bindings))

	for index, binding := range spec.Bindings {
		paths[index] = resourcePathKey(binding.Resource)
	}

	return paths
}

// ResourcePath returns the resource path key of a resource document, ignoring unmodeled fields.
func (Spec[E, R]) ResourcePath(document map[string]any) (string, error) {
	return ResourcePath[R](document)
}

// ResourcePath returns the resource path key of a document of resource R, empty when R has no resource path.
func ResourcePath[R any](document map[string]any) (string, error) {
	resource, err := utils.ConvertYAML[R](document, false)

	if err != nil {
		return "", err
	}

	return resourcePathKey(resource), nil
}

// resourcePathKey joins the resource path with NULL, which source identifiers can't hold.
func resourcePathKey(resource any) string {
	pather, isPather := resource.(ResourcePather)

	if !isPather {
		return ""
	}

	return strings.Join(pather.ResourcePath(), "\x00")
}

// CaptureSpec marks Spec as a capture spec, satisfying catalog.CaptureSpec.
func (Spec[E, R]) CaptureSpec() {}

// AddsNewBindings reports whether auto discovery adds unbound resources, satisfying catalog.NewBindingsAdder.
func (spec Spec[E, R]) AddsNewBindings() bool {
	return spec.AutoDiscover != nil && spec.AutoDiscover.AddNewBindings
}

// GeneratesBindings reports whether auto discovery may add bindings, satisfying catalog.BindingsGenerator.
func (spec Spec[E, R]) GeneratesBindings() bool {
	return spec.AutoDiscover != nil
}
