package flowctl

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"gopkg.in/yaml.v3"
)

// recordingFlowctl records each call, writes the pulled spec to --target and copies published --source files.
const recordingFlowctl string = `#!/bin/sh
line=""
previous=""
for argument in "$@"; do
	line="$line[$argument]"
	if [ "$previous" = "--target" ]; then cat "$FLOWCTL_PULLED" > "$argument"; fi
	if [ "$previous" = "--source" ] && [ "$2" = "publish" ]; then cp "$argument" "$FLOWCTL_PUBLISHED"; fi
	previous="$argument"
done
echo "$line" >> "$FLOWCTL_CALLS"
`

// dashName starts with a dash, which flowctl would read as a flag if passed as its own argument.
const dashName string = "-acmeCo/orders"

type recording struct {
	calls     string
	published string
	temporary string
}

func recordFlowctl(test *testing.T) (*Client, recording) {
	var directory string = test.TempDir()
	var client *Client = fakeFlowctl(test, recordingFlowctl)
	var record recording = recording{
		calls:     filepath.Join(directory, "calls"),
		published: filepath.Join(directory, "published.yaml"),
		temporary: test.TempDir(),
	}
	var pulled string = filepath.Join(directory, "pulled.yaml")

	test.Setenv("FLOWCTL_CALLS", record.calls)
	test.Setenv("FLOWCTL_PUBLISHED", record.published)
	test.Setenv("FLOWCTL_PULLED", pulled)
	test.Setenv("TMPDIR", record.temporary)

	if err := os.WriteFile(pulled, []byte("collections:\n  "+dashName+":\n    key: [/id]\n    expectPubId: \"0011\"\n"), 0600); err != nil {
		test.Fatal(err)
	}

	client.dataPlane = "ops/dp/private/acmeCo/example-data-plane"

	return client, record
}

// commandLines returns the recorded calls with workspace paths as $WORKSPACE.
func (record recording) commandLines(test *testing.T) []string {
	content, err := os.ReadFile(record.calls)

	if err != nil {
		test.Fatal(err)
	}

	var workspace *regexp.Regexp = regexp.MustCompile(regexp.QuoteMeta(filepath.Clean(record.temporary)) + "/estuary-tf-[0-9]+")

	return strings.Split(strings.TrimSpace(workspace.ReplaceAllString(string(content), "$$WORKSPACE")), "\n")
}

func assertCommandLines(test *testing.T, got []string, want []string) {
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		test.Errorf("calls =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestPublishCaptureCommandLines(test *testing.T) {
	client, record := recordFlowctl(test)
	var spec catalog.CaptureSpec = catalog.RawCapture(map[string]any{
		"endpoint": map[string]any{"connector": map[string]any{"image": "ghcr.io/estuary/source-mongodb:v3", "config": map[string]any{"address": "mongodb://db"}}},
		"bindings": []any{map[string]any{"resource": map[string]any{"collection": "orders"}, "target": dashName}},
	})

	if err := client.PublishCapture(context.Background(), "acmeCo/source-mongodb", spec, []string{dashName}); err != nil {
		test.Fatal(err)
	}

	assertCommandLines(test, record.commandLines(test), []string{
		"[catalog][pull-specs][--name=" + dashName + "][--flat][--overwrite][--target][$WORKSPACE/resets.flow.yaml]",
		"[catalog][publish][--source][$WORKSPACE/flow.yaml][--auto-approve][--init-data-plane=ops/dp/private/acmeCo/example-data-plane]",
	})

	content, err := os.ReadFile(record.published)

	if err != nil {
		test.Fatal(err)
	}

	var published map[string]any

	if err := yaml.Unmarshal(content, &published); err != nil {
		test.Fatal(err)
	}

	if utils.Lookup(published, "captures", "acmeCo/source-mongodb", "endpoint", "connector", "config")["address"] != "mongodb://db" {
		test.Errorf("published spec lost the capture: %s", content)
	}

	if reset := utils.Lookup(published, "collections", dashName); reset["reset"] != true || reset[catalog.ExpectPubIDKey] != nil {
		test.Errorf("published spec lost the reset collection: %s", content)
	}
}

func TestPublishMaterializationCommandLines(test *testing.T) {
	client, record := recordFlowctl(test)
	var spec catalog.MaterializationSpec = catalog.RawMaterialization(map[string]any{
		"endpoint": map[string]any{"connector": map[string]any{"image": "ghcr.io/estuary/materialize-postgres:v4", "config": map[string]any{"address": "db:5432"}}},
		"bindings": []any{map[string]any{"resource": map[string]any{"table": "orders"}, "source": dashName}},
	})

	if err := client.PublishMaterialization(context.Background(), "acmeCo/materialize-postgres", spec); err != nil {
		test.Fatal(err)
	}

	assertCommandLines(test, record.commandLines(test), []string{
		"[catalog][publish][--source][$WORKSPACE/flow.yaml][--auto-approve][--init-data-plane=ops/dp/private/acmeCo/example-data-plane]",
	})

	if content, err := os.ReadFile(record.published); err != nil || !strings.Contains(string(content), "acmeCo/materialize-postgres:") {
		test.Errorf("published spec = %s, %v, want the materialization", content, err)
	}
}

func TestReadDeleteAndDiscoverCommandLines(test *testing.T) {
	client, record := recordFlowctl(test)
	var spec catalog.CaptureSpec = catalog.RawCapture(map[string]any{"bindings": []any{}})
	var pulled string = filepath.Join(test.TempDir(), "pulled.yaml")

	if err := os.WriteFile(pulled, nil, 0600); err != nil {
		test.Fatal(err)
	}

	test.Setenv("FLOWCTL_PULLED", pulled)

	if _, err := client.ReadSpec(context.Background(), dashName); err != nil {
		test.Fatal(err)
	}

	if err := client.DeleteCatalogEntry(context.Background(), dashName); err != nil {
		test.Fatal(err)
	}

	if _, err := client.DiscoverCapture(context.Background(), dashName, spec); err != nil {
		test.Fatal(err)
	}

	assertCommandLines(test, record.commandLines(test), []string{
		"[catalog][pull-specs][--name=" + dashName + "][--flat][--overwrite][--target][$WORKSPACE/flow.yaml]",
		"[catalog][delete][--name=" + dashName + "][--dangerous-auto-approve]",
		"[discover][--flat][--data-plane=ops/dp/private/acmeCo/example-data-plane][--source][$WORKSPACE/flow.yaml]",
	})
}

func TestCommandLinesWithoutDataPlane(test *testing.T) {
	client, record := recordFlowctl(test)

	client.dataPlane = ""

	if err := client.PublishMaterialization(context.Background(), "acmeCo/materialize-postgres", catalog.RawMaterialization(map[string]any{})); err != nil {
		test.Fatal(err)
	}

	assertCommandLines(test, record.commandLines(test), []string{
		"[catalog][publish][--source][$WORKSPACE/flow.yaml][--auto-approve]",
	})
}
