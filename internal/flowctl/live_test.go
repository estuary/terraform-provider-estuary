package flowctl

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
)

// liveClient returns a client of the real flowctl, skipping the test without FLOWCTL_LIVE_BINARY, FLOW_AUTH_TOKEN or
// variable.
func liveClient(test *testing.T, variable string) (*Client, string) {
	var binary string = os.Getenv("FLOWCTL_LIVE_BINARY")
	var value string = os.Getenv(variable)

	if binary == "" || os.Getenv("FLOW_AUTH_TOKEN") == "" || value == "" {
		test.Skipf("set FLOWCTL_LIVE_BINARY, FLOW_AUTH_TOKEN and %s to run against the real flowctl", variable)
	}

	return NewClient(binary, os.Getenv("FLOW_AUTH_TOKEN"), os.Getenv("FLOWCTL_LIVE_DATA_PLANE")), value
}

// logTree logs the files flowctl wrote and the tree read, so the layout can be checked by eye.
func logTree(test *testing.T, tree *catalogTree) {
	_ = filepath.WalkDir(tree.directory, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			relative, _ := filepath.Rel(tree.directory, path)

			test.Logf("written: %s", relative)
		}

		return nil
	})

	test.Logf("read through imports: %s", strings.Join(tree.paths, ", "))
	test.Logf("entries: %s", strings.Join(tree.names(), ", "))
}

// assertSchemasResolve checks that each schema file reference of the collection resolves to a file of the workspace.
func assertSchemasResolve(test *testing.T, tree *catalogTree, name string) {
	collection, file, found := tree.entry(collectionsKey, name)

	if !found {
		test.Errorf("collection %s isn't in the tree", name)

		return
	}

	root, err := os.OpenRoot(tree.directory)

	if err != nil {
		test.Fatal(err)
	}

	defer root.Close()

	for _, key := range []string{"schema", "writeSchema", "readSchema"} {
		reference, isPath := collection[key].(string)

		if !isPath {
			continue
		}

		schemaFile, err := resolve(file, reference)

		if err == nil {
			_, err = root.Stat(schemaFile)
		}

		if err != nil {
			test.Errorf("%s %s = %s, in %s: %v", name, key, reference, file, err)

			continue
		}

		test.Logf("%s %s -> %s", name, key, schemaFile)
	}
}

func TestLiveReadSpec(test *testing.T) {
	client, name := liveClient(test, "FLOWCTL_LIVE_TASK")

	spec, err := client.ReadSpec(context.Background(), name)

	if err != nil || spec == nil {
		test.Fatalf("ReadSpec(%s) = %v, %v, want the live spec", name, spec != nil, err)
	}

	var config map[string]any

	for _, endpoint := range utils.Lookup(spec, catalog.EndpointKey) {
		object, _ := endpoint.(map[string]any)
		config, _ = object[catalog.ConfigKey].(map[string]any)
	}

	if config == nil {
		test.Errorf("ReadSpec(%s) endpoint config = %v, want the config file inlined", name, utils.Lookup(spec, catalog.EndpointKey))
	}

	// Keys only: the values may be secrets.
	test.Logf("%s: %d bindings, config keys %v", name, len(catalog.Bindings(spec)), slices.Sorted(func(yield func(string) bool) {
		for key := range config {
			if !yield(key) {
				return
			}
		}
	}))

	if deleted, err := client.ReadSpec(context.Background(), name+"-does-not-exist"); err != nil || deleted != nil {
		test.Errorf("ReadSpec of a missing name = %v, %v, want nil, nil as for a deleted entry", deleted != nil, err)
	}
}

func TestLivePullCollections(test *testing.T) {
	client, value := liveClient(test, "FLOWCTL_LIVE_COLLECTIONS")
	var names []string = strings.Split(value, ",")

	tree, err := client.pull(context.Background(), test.TempDir(), names...)

	if err != nil {
		test.Fatalf("pull: %v", err)
	}

	logTree(test, tree)

	for _, name := range names {
		assertSchemasResolve(test, tree, name)
	}

	if err := markResets(tree, names); err != nil {
		test.Errorf("markResets: %v", err)
	}
}

func TestLiveDiscover(test *testing.T) {
	client, specPath := liveClient(test, "FLOWCTL_LIVE_DISCOVER_SPEC")
	var name string = os.Getenv("FLOWCTL_LIVE_DISCOVER_NAME")

	content, err := os.ReadFile(specPath)

	if err != nil || name == "" {
		test.Fatalf("reading FLOWCTL_LIVE_DISCOVER_SPEC: %v, FLOWCTL_LIVE_DISCOVER_NAME = %q", err, name)
	}

	err = inWorkspace(map[string][]byte{specFile: content}, func(directory string) error {
		tree, err := client.discover(context.Background(), directory)

		if err != nil {
			return err
		}

		logTree(test, tree)

		var capture map[string]any = tree.capture(name)

		if capture == nil {
			test.Fatalf("discovered tree has no capture %s", name)
		}

		for _, binding := range catalog.Bindings(capture) {
			assertSchemasResolve(test, tree, catalog.BindingCollection(binding))
		}

		test.Logf("%s: %d discovered bindings", name, len(catalog.Bindings(capture)))

		return nil
	})

	if err != nil {
		test.Fatalf("discover: %v", err)
	}
}
