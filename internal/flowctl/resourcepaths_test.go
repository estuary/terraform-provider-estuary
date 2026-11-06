package flowctl

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

// homeFlowctl records the HOME and XDG_CONFIG_HOME of each call.
const homeFlowctl string = `#!/bin/sh
echo "$HOME $XDG_CONFIG_HOME" >> "$FLOWCTL_CALLS"
`

func TestResourcePathsCommandLines(test *testing.T) {
	client, record := recordFlowctl(test)
	var validated string = filepath.Join(test.TempDir(), "validated.yaml")
	var spec catalog.MaterializationSpec = catalog.RawMaterialization(map[string]any{
		"endpoint": map[string]any{"connector": map[string]any{"image": "ghcr.io/estuary/materialize-postgres:v4", "config": map[string]any{"address": "db:5432"}}},
		"bindings": []any{
			map[string]any{"resource": map[string]any{"table": "orders"}, "source": "acmeCo/orders"},
			map[string]any{"resource": map[string]any{"table": "orders_history"}, "source": "acmeCo/orders"},
		},
	})

	if err := os.WriteFile(validated, []byte(strings.Join([]string{
		"materializations:",
		"  acmeCo/materialize-postgres:",
		"    bindings:",
		"    - resource: {_meta: {path: [public, orders]}, table: orders}",
		"      source: acmeCo/orders",
		"    - resource: {_meta: {path: [public, orders_history]}, table: orders_history}",
		"      source: acmeCo/orders",
	}, "\n")), 0600); err != nil {
		test.Fatal(err)
	}

	test.Setenv("FLOWCTL_PULLED", validated)

	paths, err := client.ResourcePaths(context.Background(), "acmeCo/materialize-postgres", spec)

	if err != nil {
		test.Fatal(err)
	}

	if want := [][]string{{"public", "orders"}, {"public", "orders_history"}}; !reflect.DeepEqual(paths, want) {
		test.Errorf("paths = %v, want %v", paths, want)
	}

	assertCommandLines(test, record.commandLines(test), []string{
		"[draft][create]",
		"[draft][author][--source][$WORKSPACE/flow.yaml]",
		"[draft][test]",
		"[draft][develop][--target][$WORKSPACE/validated/flow.yaml]",
		"[draft][delete]",
	})
}

func TestResourcePathsDeletesTheDraftOnFailure(test *testing.T) {
	client, record := recordFlowctl(test)
	var validated string = filepath.Join(test.TempDir(), "validated.yaml")
	var spec catalog.MaterializationSpec = catalog.RawMaterialization(map[string]any{
		"bindings": []any{map[string]any{"resource": map[string]any{"table": "orders"}, "source": "acmeCo/orders"}},
	})

	if err := os.WriteFile(validated, []byte("materializations:\n  acmeCo/materialize-postgres:\n    bindings: []\n"), 0600); err != nil {
		test.Fatal(err)
	}

	test.Setenv("FLOWCTL_PULLED", validated)

	if _, err := client.ResourcePaths(context.Background(), "acmeCo/materialize-postgres", spec); err == nil || !strings.Contains(err.Error(), "has 0 bindings, not the 1 validated") {
		test.Errorf("error = %v, want the missing binding reported", err)
	}

	if lines := record.commandLines(test); lines[len(lines)-1] != "[draft][delete]" {
		test.Errorf("calls = %v, want the draft deleted", lines)
	}
}

func TestResourcePathsUsesAPrivateFlowctlHome(test *testing.T) {
	var calls string = filepath.Join(test.TempDir(), "calls")
	var temporary string = test.TempDir()
	var client *Client = fakeFlowctl(test, homeFlowctl)

	test.Setenv("FLOWCTL_CALLS", calls)
	test.Setenv("TMPDIR", temporary)

	_, _ = client.ResourcePaths(context.Background(), "acmeCo/materialize-postgres", catalog.RawMaterialization(map[string]any{"bindings": []any{}}))

	content, err := os.ReadFile(calls)

	if err != nil {
		test.Fatal(err)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		home, config, _ := strings.Cut(line, " ")

		if !strings.HasPrefix(home, filepath.Clean(temporary)) || !strings.HasSuffix(home, "/"+draftHomeDirectory) || config != home+"/.config" {
			test.Errorf("flowctl ran with HOME %q and XDG_CONFIG_HOME %q, want a private home in the workspace", home, config)
		}
	}
}

func TestValidatedPathsRejectsBindingsWithoutPath(test *testing.T) {
	var tree *catalogTree = &catalogTree{
		paths: []string{specFile},
		files: map[string]map[string]any{specFile: {
			materializationsKey: map[string]any{"acmeCo/materialize-postgres": map[string]any{
				"bindings": []any{map[string]any{"resource": map[string]any{"table": "orders"}, "source": "acmeCo/orders"}},
			}},
		}},
	}

	if _, err := validatedPaths(tree, "acmeCo/materialize-postgres", 1); err == nil || !strings.Contains(err.Error(), "has no resource path") {
		test.Errorf("error = %v, want the binding without resource path rejected", err)
	}

	if _, err := validatedPaths(tree, "acmeCo/other", 1); err == nil || !strings.Contains(err.Error(), "has no materialization acmeCo/other") {
		test.Errorf("error = %v, want the missing materialization rejected", err)
	}
}
