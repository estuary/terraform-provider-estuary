package spectest

import (
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"gopkg.in/yaml.v3"
)

// unknownKey is added to the samples to check strict decoding rejects keys without a mapped attribute.
const unknownKey string = "unmappedKey"

// RunCaptureSuite runs the checks every capture connector gets over its samples: strict decoding, round
// trip, the connector image and the rejection of unknown config and resource keys. S is the connector
// capture spec and image the connector image.
func RunCaptureSuite[S any](test *testing.T, image string) {
	test.Helper()

	runSuite(test, "captures", image, RawCaptures, ParseCaptures[S])
}

// RunMaterializationSuite runs the checks every materialization connector gets over its samples, as
// RunCaptureSuite does. An empty image skips the image check, for endpoints without one such as Dekaf.
func RunMaterializationSuite[S any](test *testing.T, image string) {
	test.Helper()

	runSuite(test, "materializations", image, RawMaterializations, ParseMaterializations[S])
}

func runSuite[S any](test *testing.T, section string, image string, raw func(testing.TB, string) map[string]any, parse func([]byte) (map[string]S, error)) {
	test.Helper()

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
			test.Run(file+"/"+name, func(test *testing.T) {
				var rawSpec map[string]any = utils.Lookup(rawContent, name)

				AssertRoundTrip(test, spec, rawSpec)

				assertImage(test, rawSpec, image)

				assertRejectsUnknownKeys(test, section, name, rawSpec, parse)
			})
		}
	}
}

func assertImage(test testing.TB, rawSpec map[string]any, image string) {
	test.Helper()

	if image == "" {
		return
	}

	var connector map[string]any = utils.Lookup(rawSpec, "endpoint", "connector")

	if connector["image"] != image {
		test.Errorf("image = %v, want %q", connector["image"], image)
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
