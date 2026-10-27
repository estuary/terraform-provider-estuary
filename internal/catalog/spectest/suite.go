package spectest

import (
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"gopkg.in/yaml.v3"
)

// unknownKey is added to the samples to check strict decoding rejects keys without a mapped attribute.
const unknownKey string = "unmappedKey"

// Connector is an image a spec decodes and the connector name ending its task names.
type Connector struct {
	Image string
	Name  string
}

// RunCaptureSuite runs the shared capture checks over the samples: strict decoding, round trip, images, names and
// rejection of unknown config and resource keys.
func RunCaptureSuite[S any](test *testing.T, connectors ...Connector) {
	test.Helper()

	var spec any = *new(S)

	if _, isCapture := spec.(catalog.CaptureSpec); !isCapture {
		test.Errorf("%T does not implement catalog.CaptureSpec", spec)
	}

	if _, isMaterialization := spec.(catalog.MaterializationSpec); isMaterialization {
		test.Errorf("%T is a capture spec implementing catalog.MaterializationSpec", spec)
	}

	runSuite(test, "captures", connectors, RawCaptures, ParseCaptures[S])
}

// RunMaterializationSuite runs the RunCaptureSuite checks for materializations. No connectors skips the image checks,
// for endpoints without one such as Dekaf.
func RunMaterializationSuite[S any](test *testing.T, connectors ...Connector) {
	test.Helper()

	var spec any = *new(S)

	if _, isMaterialization := spec.(catalog.MaterializationSpec); !isMaterialization {
		test.Errorf("%T does not implement catalog.MaterializationSpec", spec)
	}

	if _, isCapture := spec.(catalog.CaptureSpec); isCapture {
		test.Errorf("%T is a materialization spec implementing catalog.CaptureSpec", spec)
	}

	runSuite(test, "materializations", connectors, RawMaterializations, ParseMaterializations[S])
}

func runSuite[S any](test *testing.T, section string, connectors []Connector, raw func(testing.TB, string) map[string]any, parse func([]byte) (map[string]S, error)) {
	test.Helper()

	var images map[string]bool = map[string]bool{}
	var seenImages map[string]bool = map[string]bool{}

	for _, connector := range connectors {
		var name string = connector.Image[strings.LastIndex(connector.Image, "/")+1:]

		if index := strings.LastIndex(name, ":"); index >= 0 {
			name = name[:index]
		}

		if connector.Name != name {
			test.Errorf("connector name %q does not match image %q", connector.Name, connector.Image)
		}

		images[connector.Image] = true
	}

	for _, file := range SampleFiles(test) {
		var rawContent map[string]any = raw(test, file)

		specs, err := parse(readFile(test, file))

		if err != nil {
			test.Fatalf("strict decoding %s: %v", file, err)
		}

		if len(specs) == 0 {
			test.Fatalf("%s has no %s", file, section)
		}

		for name, spec := range specs {
			var rawSpec map[string]any = utils.Lookup(rawContent, name)

			image, _ := utils.Lookup(rawSpec, "endpoint", "connector")["image"].(string)

			seenImages[image] = true

			test.Run(file+"/"+name, func(test *testing.T) {
				AssertRoundTrip(test, spec, rawSpec)

				if len(images) > 0 && !images[image] {
					test.Errorf("image = %q, want one of the connector images", image)
				}

				assertRejectsUnknownKeys(test, section, name, rawSpec, parse)
			})
		}
	}

	for _, connector := range connectors {
		if !seenImages[connector.Image] {
			test.Errorf("image %q has no sample", connector.Image)
		}
	}
}

func assertRejectsUnknownKeys[S any](test testing.TB, section string, name string, rawSpec map[string]any, parse func([]byte) (map[string]S, error)) {
	test.Helper()

	targets := map[string]func(map[string]any) map[string]any{
		"config":   endpointConfig,
		"resource": firstResource,
	}

	for target, find := range targets {
		var spec map[string]any = deepCopy(test, rawSpec)

		var object map[string]any = find(spec)

		if object == nil {
			continue
		}

		object[unknownKey] = true

		content, err := yaml.Marshal(map[string]any{section: map[string]any{name: spec}})

		if err != nil {
			test.Fatalf("marshaling spec with an unknown %s key: %v", target, err)
		}

		_, err = parse(content)

		if err == nil || !strings.Contains(err.Error(), "field "+unknownKey+" not found") {
			test.Errorf("error = %v, want strict decoding to reject an unknown %s key", err, target)
		}
	}
}

func endpointConfig(spec map[string]any) map[string]any {
	for _, endpoint := range utils.Lookup(spec, "endpoint") {
		return utils.Lookup(endpoint, "config")
	}

	return nil
}

func firstResource(spec map[string]any) map[string]any {
	bindings, _ := spec["bindings"].([]any)

	if len(bindings) == 0 {
		return nil
	}

	return utils.Lookup(bindings[0], "resource")
}

func deepCopy(test testing.TB, value map[string]any) map[string]any {
	test.Helper()

	copied, err := utils.YAMLDocument(value)

	if err != nil {
		test.Fatalf("copying spec: %v", err)
	}

	return copied
}
