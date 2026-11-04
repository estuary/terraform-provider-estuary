package flowctl

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"gopkg.in/yaml.v3"
)

// importKey lists the spec files a spec file imports, relative to its own directory.
const importKey string = "import"

// Entry kinds of a spec file.
const (
	capturesKey         string = "captures"
	collectionsKey      string = "collections"
	materializationsKey string = "materializations"
)

// catalogTree is the root spec file of a workspace and every file it imports, in flowctl's nested layout. Never use
// --flat: it names side files by the last name segment, so acmeCo/mysql/sales and acmeCo/postgres/sales collide.
type catalogTree struct {
	directory string
	paths     []string                  // slash paths relative to directory, in reading order
	files     map[string]map[string]any // parsed content by path
}

// readCatalogTree reads the workspace spec file and every file it imports, an empty tree when flowctl wrote no spec
// file, as it does when pulling nothing. References out of the workspace are rejected.
func readCatalogTree(directory string) (*catalogTree, error) {
	var tree *catalogTree = &catalogTree{directory: directory, files: map[string]map[string]any{}}

	root, err := os.OpenRoot(directory)

	if err != nil {
		return nil, fmt.Errorf("opening workspace: %w", err)
	}

	defer root.Close()

	if _, err := root.Stat(specFile); errors.Is(err, fs.ErrNotExist) {
		return tree, nil
	}

	return tree, tree.read(root, specFile)
}

func (tree *catalogTree) read(root *os.Root, file string) error {
	if _, read := tree.files[file]; read {
		return nil
	}

	var document map[string]any = map[string]any{}

	if err := readYAML(root, file, &document); err != nil {
		return err
	}

	if document == nil {
		document = map[string]any{}
	}

	tree.files[file] = document
	tree.paths = append(tree.paths, file)

	imports, _ := document[importKey].([]any)

	for _, imported := range imports {
		reference, err := resolve(file, imported)

		if err != nil {
			return fmt.Errorf("%s imports %w", file, err)
		}

		if err := tree.read(root, reference); err != nil {
			return err
		}
	}

	return nil
}

// resolve returns the workspace path of a file reference written in file, rejecting non-string references and paths
// out of the workspace.
func resolve(file string, reference any) (string, error) {
	relative, isPath := reference.(string)

	if !isPath || path.IsAbs(relative) {
		return "", fmt.Errorf("%v, out of the workspace", reference)
	}

	var joined string = path.Join(path.Dir(file), relative)

	if !filepath.IsLocal(joined) {
		return "", fmt.Errorf("%v, out of the workspace", reference)
	}

	return joined, nil
}

// readYAML parses the workspace file into value, through root so that symbolic links can't leave the workspace.
func readYAML(root *os.Root, file string, value any) error {
	content, err := root.ReadFile(file)

	if err != nil {
		return fmt.Errorf("reading %s: %w", file, err)
	}

	if err := yaml.Unmarshal(content, value); err != nil {
		return fmt.Errorf("parsing %s: %w", file, err)
	}

	return nil
}

// entry returns the spec of name among the entries of kind, the file holding it, and whether any file holds it, even
// as null.
func (tree *catalogTree) entry(kind string, name string) (map[string]any, string, bool) {
	for _, file := range tree.paths {
		entries, _ := tree.files[file][kind].(map[string]any)

		if value, found := entries[name]; found {
			spec, _ := value.(map[string]any)

			return spec, file, true
		}
	}

	return nil, "", false
}

// capture returns the capture spec of name, nil when no file holds it.
func (tree *catalogTree) capture(name string) map[string]any {
	spec, _, _ := tree.entry(capturesKey, name)

	return spec
}

// collections returns the collections of the file holding the collection name, nil when no file holds it. Renaming a
// collection in it keeps the collection in its file.
func (tree *catalogTree) collections(name string) map[string]any {
	if _, file, found := tree.entry(collectionsKey, name); found {
		return tree.files[file][collectionsKey].(map[string]any)
	}

	return nil
}

// names returns the sorted names of every capture, collection and materialization of the tree.
func (tree *catalogTree) names() []string {
	var names []string

	for _, file := range tree.paths {
		for _, kind := range []string{capturesKey, collectionsKey, materializationsKey} {
			entries, _ := tree.files[file][kind].(map[string]any)

			names = slices.AppendSeq(names, maps.Keys(entries))
		}
	}

	slices.Sort(names)

	return names
}

// inlineConfigs replaces each endpoint config file reference of spec, written in file, with the file content.
func (tree *catalogTree) inlineConfigs(spec map[string]any, file string) error {
	root, err := os.OpenRoot(tree.directory)

	if err != nil {
		return fmt.Errorf("opening workspace: %w", err)
	}

	defer root.Close()

	endpoints, _ := spec[catalog.EndpointKey].(map[string]any)

	for _, endpoint := range endpoints {
		object, _ := endpoint.(map[string]any)
		reference, isPath := object[catalog.ConfigKey].(string)

		if !isPath {
			continue
		}

		configFile, err := resolve(file, reference)

		if err != nil {
			return fmt.Errorf("pulled config %w", err)
		}

		var config any

		if err := readYAML(root, configFile, &config); err != nil {
			return fmt.Errorf("pulled config: %w", err)
		}

		object[catalog.ConfigKey] = config
	}

	return nil
}

// write writes every file of the tree back to the workspace.
func (tree *catalogTree) write() error {
	root, err := os.OpenRoot(tree.directory)

	if err != nil {
		return fmt.Errorf("opening workspace: %w", err)
	}

	defer root.Close()

	for _, file := range tree.paths {
		content, err := yaml.Marshal(tree.files[file])

		if err != nil {
			return fmt.Errorf("marshaling %s: %w", file, err)
		}

		if err := root.WriteFile(file, content, 0600); err != nil {
			return fmt.Errorf("writing %s: %w", file, err)
		}
	}

	return nil
}
