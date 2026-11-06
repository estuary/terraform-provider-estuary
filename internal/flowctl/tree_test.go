package flowctl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree writes the spec files, by slash path relative to a new workspace, and reads them as a catalog tree.
func writeTree(test *testing.T, files map[string]string) *catalogTree {
	test.Helper()

	var directory string = test.TempDir()

	for file, content := range files {
		var path string = filepath.Join(directory, filepath.FromSlash(file))

		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			test.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			test.Fatal(err)
		}
	}

	tree, err := readCatalogTree(directory)

	if err != nil {
		test.Fatalf("readCatalogTree: %v", err)
	}

	return tree
}

func TestCatalogTreeFollowsImports(test *testing.T) {
	var tree *catalogTree = writeTree(test, map[string]string{
		"flow.yaml":                   "import: [acmeCo/flow.yaml]\n",
		"acmeCo/flow.yaml":            "import: [mysql/flow.yaml, postgres/flow.yaml]\ncaptures:\n  acmeCo/source-mysql: { bindings: [] }\n",
		"acmeCo/mysql/flow.yaml":      "collections:\n  acmeCo/mysql/sales: { key: [/id], writeSchema: sales.write.schema.yaml }\n",
		"acmeCo/postgres/flow.yaml":   "collections:\n  acmeCo/postgres/sales: { key: [/id], writeSchema: sales.write.schema.yaml }\n",
		"acmeCo/unimported/flow.yaml": "collections:\n  acmeCo/unimported: { key: [/id] }\n",
	})

	if tree.capture("acmeCo/source-mysql") == nil || tree.capture("acmeCo/missing") != nil {
		test.Errorf("capture lookup = %v, want the capture of the imported file only", tree.files)
	}

	for _, name := range []string{"acmeCo/mysql/sales", "acmeCo/postgres/sales"} {
		if tree.collections(name) == nil {
			test.Errorf("collections(%s) = nil, want the imported file holding it", name)
		}
	}

	if tree.collections("acmeCo/unimported") != nil {
		test.Error("collections read a file no spec file imports")
	}

	tree.collections("acmeCo/mysql/sales")["acmeCo/mysql/sales"].(map[string]any)["reset"] = true

	if err := tree.write(); err != nil {
		test.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tree.directory, "acmeCo", "mysql", "flow.yaml"))

	if err != nil || !strings.Contains(string(content), "reset: true") || !strings.Contains(string(content), "writeSchema: sales.write.schema.yaml") {
		test.Errorf("written acmeCo/mysql/flow.yaml = %s, %v, want the change in the file holding the collection", content, err)
	}
}

func TestCatalogTreeRejectsImportsOutOfTheWorkspace(test *testing.T) {
	for _, imported := range []string{"../outside.flow.yaml", "/etc/outside.flow.yaml", "{ url: flow.yaml }"} {
		var directory string = test.TempDir()

		if err := os.WriteFile(filepath.Join(directory, specFile), []byte("import: ["+imported+"]\n"), 0600); err != nil {
			test.Fatal(err)
		}

		if tree, err := readCatalogTree(directory); err == nil {
			test.Errorf("%s: readCatalogTree = %v, want an error", imported, tree.paths)
		}
	}
}
