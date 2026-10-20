package spectest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// SampleFiles returns every YAML sample under the package testdata directory.
func SampleFiles(test testing.TB) []string {
	test.Helper()

	files, err := filepath.Glob(filepath.Join("testdata", "*.yaml"))

	if err != nil {
		test.Fatalf("listing testdata samples: %v", err)
	}

	if len(files) == 0 {
		test.Fatal("no testdata samples found")
	}

	return files
}

// DecodeStrict decodes a YAML document into out, failing on any key without a mapped attribute.
func DecodeStrict(content []byte, out any) error {
	var decoder *yaml.Decoder = yaml.NewDecoder(bytes.NewReader(Dedent(content)))

	decoder.KnownFields(true)

	return decoder.Decode(out)
}

// Dedent strips the shared leading tabs of a tab indented YAML document, each remaining tab becoming two spaces.
func Dedent(content []byte) []byte {
	var lines [][]byte = bytes.Split(content, []byte("\n"))

	var shared int = -1

	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var tabs int = len(line) - len(bytes.TrimLeft(line, "\t"))

		if shared == -1 || tabs < shared {
			shared = tabs
		}
	}

	if shared <= 0 && !bytes.Contains(content, []byte("\n\t")) {
		return content
	}

	for index, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			lines[index] = nil

			continue
		}

		var tabs int = len(line) - len(bytes.TrimLeft(line, "\t"))

		lines[index] = append(bytes.Repeat([]byte("  "), tabs-shared), line[tabs:]...)
	}

	return bytes.Join(lines, []byte("\n"))
}

// ParseCaptures strictly decodes a catalog spec document into its captures, S being the connector capture spec.
func ParseCaptures[S any](content []byte) (map[string]S, error) {
	var document struct {
		Captures map[string]S `yaml:"captures"`
	}

	var err error = DecodeStrict(content, &document)

	return document.Captures, err
}

// ParseMaterializations strictly decodes a catalog spec document into its materializations, S being the
// connector materialization spec.
func ParseMaterializations[S any](content []byte) (map[string]S, error) {
	var document struct {
		Materializations map[string]S `yaml:"materializations"`
	}

	var err error = DecodeStrict(content, &document)

	return document.Materializations, err
}

// DecodeCaptures reads a YAML file and strictly decodes its captures, failing the test on error.
func DecodeCaptures[S any](test testing.TB, path string) map[string]S {
	test.Helper()

	captures, err := ParseCaptures[S](readFile(test, path))

	if err != nil {
		test.Fatalf("strict decoding %s: %v", path, err)
	}

	return captures
}

// DecodeMaterializations reads a YAML file and strictly decodes its materializations, failing the test on error.
func DecodeMaterializations[S any](test testing.TB, path string) map[string]S {
	test.Helper()

	materializations, err := ParseMaterializations[S](readFile(test, path))

	if err != nil {
		test.Fatalf("strict decoding %s: %v", path, err)
	}

	return materializations
}

func readFile(test testing.TB, path string) []byte {
	test.Helper()

	content, err := os.ReadFile(path)

	if err != nil {
		test.Fatalf("reading %s: %v", path, err)
	}

	return content
}
