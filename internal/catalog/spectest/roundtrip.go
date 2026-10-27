package spectest

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"
)

// RawMaterializations returns the untyped materializations of a YAML file.
func RawMaterializations(test testing.TB, path string) map[string]any {
	test.Helper()

	return rawSections(test, path).Materializations
}

// RawCaptures returns the untyped captures of a YAML file.
func RawCaptures(test testing.TB, path string) map[string]any {
	test.Helper()

	return rawSections(test, path).Captures
}

type sections struct {
	Captures         map[string]any `yaml:"captures"`
	Materializations map[string]any `yaml:"materializations"`
}

func rawSections(test testing.TB, path string) sections {
	test.Helper()

	var document sections

	if err := yaml.Unmarshal(readFile(test, path), &document); err != nil {
		test.Fatalf("decoding %s: %v", path, err)
	}

	return document
}

// AssertRoundTrip checks spec marshals back to want, its source sample. Keys false in want may be omitted.
func AssertRoundTrip(test testing.TB, spec any, want any) {
	test.Helper()

	var got any

	content, err := yaml.Marshal(spec)

	if err != nil {
		test.Fatalf("marshaling spec: %v", err)
	}

	if err := yaml.Unmarshal(content, &got); err != nil {
		test.Fatalf("decoding marshaled spec: %v", err)
	}

	if differences := diff("", want, got); len(differences) > 0 {
		test.Errorf("round trip differs from sample:\n%s\nmarshaled spec:\n%s", joinLines(differences), content)
	}
}

func diff(path string, want any, got any) []string {
	wantMap, wantIsMap := want.(map[string]any)

	gotMap, gotIsMap := got.(map[string]any)

	if wantIsMap && gotIsMap {
		var differences []string

		for _, key := range sortedKeys(wantMap) {
			gotValue, found := gotMap[key]

			if !found {
				if wantMap[key] != false {
					differences = append(differences, fmt.Sprintf("%s/%s: missing, want %v", path, key, wantMap[key]))
				}

				continue
			}

			differences = append(differences, diff(path+"/"+key, wantMap[key], gotValue)...)
		}

		for _, key := range sortedKeys(gotMap) {
			if _, found := wantMap[key]; !found {
				differences = append(differences, fmt.Sprintf("%s/%s: unexpected %v", path, key, gotMap[key]))
			}
		}

		return differences
	}

	wantList, wantIsList := want.([]any)

	gotList, gotIsList := got.([]any)

	if wantIsList && gotIsList && len(wantList) == len(gotList) {
		var differences []string

		for index := range wantList {
			differences = append(differences, diff(fmt.Sprintf("%s/%d", path, index), wantList[index], gotList[index])...)
		}

		return differences
	}

	if !reflect.DeepEqual(want, got) {
		return []string{fmt.Sprintf("%s: got %v, want %v", path, got, want)}
	}

	return nil
}

func sortedKeys(values map[string]any) []string {
	var keys []string = make([]string, 0, len(values))

	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func joinLines(lines []string) string {
	var buffer bytes.Buffer

	for _, line := range lines {
		buffer.WriteString("  ")
		buffer.WriteString(line)
		buffer.WriteString("\n")
	}

	return buffer.String()
}
