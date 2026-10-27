package materialization

import (
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"gopkg.in/yaml.v3"
)

type testResource struct {
	Table string `yaml:"table"`
}

func TestSpecsGenerateBindings(test *testing.T) {
	var sourceCapture string = "acmeCo/movies/source-mongodb"

	expectations := map[string]struct {
		spec Spec[struct{}, testResource]
		want bool
	}{
		"with source capture": {Spec[struct{}, testResource]{SpecOptions: SpecOptions{Source: &Source{Capture: &sourceCapture}}}, true},
		"with null capture":   {Spec[struct{}, testResource]{SpecOptions: SpecOptions{Source: &Source{DeltaUpdates: true}}}, false},
		"without source":      {Spec[struct{}, testResource]{}, false},
	}

	for name, expected := range expectations {
		var generator catalog.BindingsGenerator = expected.spec

		if got := generator.GeneratesBindings(); got != expected.want {
			test.Errorf("%s: GeneratesBindings = %v, want %v", name, got, expected.want)
		}
	}
}

func TestFieldDepthYAML(test *testing.T) {
	expectations := map[string]struct {
		written string
		depth   FieldDepth
		want    string
	}{
		"number":        {"1", "1", "1\n"},
		"true":          {"true", FieldDepthUnlimited, "true\n"},
		"false":         {"false", "0", "0\n"},
		"own text":      {"unlimited", FieldDepthUnlimited, "true\n"},
		"quoted number": {`"2"`, "2", "2\n"},
	}

	for name, expected := range expectations {
		var depth FieldDepth

		if err := yaml.Unmarshal([]byte(expected.written), &depth); err != nil || depth != expected.depth {
			test.Errorf("%s: depth = %q (%v), want %q", name, depth, err, expected.depth)
			continue
		}

		content, err := yaml.Marshal(depth)

		if err != nil || string(content) != expected.want {
			test.Errorf("%s: written %q (%v), want %q", name, content, err, expected.want)
		}
	}

	for _, invalid := range []string{"-1", "deep", "1.5"} {
		var depth FieldDepth

		if err := yaml.Unmarshal([]byte(invalid), &depth); err == nil {
			test.Errorf("depth %s accepted, want an error", invalid)
		}
	}
}

func TestBindingSourceYAML(test *testing.T) {
	expectations := map[string]struct {
		written string
		source  BindingSource
		want    string
	}{
		"name":    {"acmeCo/orders", BindingSource{Name: "acmeCo/orders"}, "acmeCo/orders\n"},
		"object":  {"{name: acmeCo/orders}", BindingSource{Name: "acmeCo/orders"}, "acmeCo/orders\n"},
		"filters": {"{name: acmeCo/orders, notBefore: 2026-01-01T00:00:00Z}", BindingSource{Name: "acmeCo/orders", NotBefore: "2026-01-01T00:00:00Z"}, "name: acmeCo/orders\nnotBefore: \"2026-01-01T00:00:00Z\"\n"},
	}

	for name, expected := range expectations {
		var source BindingSource

		if err := yaml.Unmarshal([]byte(expected.written), &source); err != nil || source != expected.source {
			test.Errorf("%s: source = %+v (%v), want %+v", name, source, err, expected.source)
			continue
		}

		content, err := yaml.Marshal(source)

		if err != nil || string(content) != expected.want {
			test.Errorf("%s: written %q (%v), want %q", name, content, err, expected.want)
		}
	}

	var source BindingSource

	if err := yaml.Unmarshal([]byte("{name: acmeCo/orders, partitions: {include: {region: [us]}}}"), &source); err == nil || !strings.Contains(err.Error(), "field partitions not found") {
		test.Errorf("error = %v, want the unmodeled partitions key rejected", err)
	}
}
