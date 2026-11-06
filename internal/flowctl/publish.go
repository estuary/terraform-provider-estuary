package flowctl

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"gopkg.in/yaml.v3"
)

const dekafConfigFile string = "dekaf.config.yaml"

// collectionRename is a discovered collection a declared binding targets under another name.
type collectionRename struct {
	previous string
	target   string
}

// withDeclaredBindings replaces discovered bindings with declared ones of the same resource, see declaredPositions,
// keeping discovered metadata. A declared binding naming another target renames the discovered collection, see
// renameCollections.
func withDeclaredBindings(tree *catalogTree, name string, declared []any, identifier catalog.BindingsIdentifier) error {
	var capture map[string]any = tree.capture(name)
	var renames []collectionRename

	if capture == nil {
		return fmt.Errorf("discovered spec has no capture %s", name)
	}

	var discovered []any = catalog.Bindings(capture)

	positions, err := declaredPositions(discovered, declared, identifier)

	if err != nil {
		return err
	}

	for index, binding := range declared {
		var position int = positions[index]

		if position == -1 {
			discovered = append(discovered, binding)

			continue
		}

		var target string = catalog.BindingCollection(binding)
		var previous string = catalog.BindingCollection(discovered[position])

		catalog.KeepResourceMeta(discovered[position], binding)

		if target != previous {
			renames = append(renames, collectionRename{previous: previous, target: target})
		}

		discovered[position] = binding
	}

	capture[catalog.BindingsKey] = discovered

	renameCollections(tree, renames, discovered)

	return nil
}

// renameCollections defines each renamed collection under its new target, where undefined, in the file defining it, in
// declaration order, so a target several collections are renamed to is defined by the first. It then removes each old
// name no binding targets, as other bindings may still share it.
func renameCollections(tree *catalogTree, renames []collectionRename, bindings []any) {
	for _, rename := range renames {
		var collections map[string]any = tree.collections(rename.previous)

		if collections == nil || tree.collections(rename.target) != nil {
			continue
		}

		var collection any = collections[rename.previous]

		if object, isObject := collection.(map[string]any); isObject {
			collection = maps.Clone(object)
		}

		collections[rename.target] = collection
	}

	for _, rename := range renames {
		var collections map[string]any = tree.collections(rename.previous)

		if collections != nil && !slices.ContainsFunc(bindings, func(binding any) bool { return catalog.BindingCollection(binding) == rename.previous }) {
			delete(collections, rename.previous)
		}
	}
}

// declaredPositions returns the discovered binding each declared one replaces, each at most once, -1 when none: the
// one with its resource path. A declared binding without one replaces the first discovered binding left without one
// whose resource it extends, else the only one left of its target while it is the only one left of that target too,
// so bindings sharing a target never swap their metadata. Several left on both sides of a target fail, as which
// replaces which is ambiguous.
func declaredPositions(discovered []any, declared []any, identifier catalog.BindingsIdentifier) ([]int, error) {
	var positions []int = make([]int, len(declared))
	var declaredPaths []string = make([]string, len(declared))
	var discoveredPaths []string = make([]string, len(discovered))
	var byPath map[string][]int = map[string][]int{}
	var taken map[int]bool = map[int]bool{}

	for position, binding := range discovered {
		discoveredPaths[position], _ = bindingPath(binding, identifier)

		if discoveredPaths[position] != "" {
			byPath[discoveredPaths[position]] = append(byPath[discoveredPaths[position]], position)
		}
	}

	for index, binding := range declared {
		path, err := bindingPath(binding, identifier)

		if err != nil {
			return nil, err
		}

		declaredPaths[index] = path
		positions[index] = -1

		if queue := byPath[path]; path != "" && len(queue) > 0 {
			positions[index] = queue[0]
			byPath[path] = queue[1:]
			taken[queue[0]] = true
		}
	}

	for index, binding := range declared {
		if declaredPaths[index] != "" {
			continue
		}

		for position, candidate := range discovered {
			if !taken[position] && discoveredPaths[position] == "" && extendsResource(binding, candidate) {
				positions[index] = position
				taken[position] = true

				break
			}
		}
	}

	var waiting map[string][]int = map[string][]int{}
	var left map[string][]int = map[string][]int{}

	for index, binding := range declared {
		if declaredPaths[index] == "" && positions[index] == -1 {
			waiting[catalog.BindingCollection(binding)] = append(waiting[catalog.BindingCollection(binding)], index)
		}
	}

	for position, binding := range discovered {
		if discoveredPaths[position] == "" && !taken[position] {
			left[catalog.BindingCollection(binding)] = append(left[catalog.BindingCollection(binding)], position)
		}
	}

	for _, target := range slices.Sorted(maps.Keys(waiting)) {
		var indexes []int = waiting[target]
		var candidates []int = left[target]

		if len(indexes) == 1 && len(candidates) == 1 {
			positions[indexes[0]] = candidates[0]
		} else if len(candidates) > 0 {
			return nil, fmt.Errorf("%d declared bindings of %s don't extend any of the %d discovered resources left for it, so which "+
				"discovered binding each replaces is ambiguous: declare each with every field of the discovered resource it replaces",
				len(indexes), target, len(candidates))
		}
	}

	return positions, nil
}

// bindingPath returns the resource path key of a binding, empty when the resource has none.
func bindingPath(binding any, identifier catalog.BindingsIdentifier) (string, error) {
	if identifier == nil {
		return "", nil
	}

	return identifier.ResourcePath(catalog.BindingResource(binding))
}

// extendsResource reports whether the resource of binding sets every field of the resource of discovered, resource
// metadata aside, to the same value. An empty discovered resource is extended by none.
func extendsResource(binding any, discovered any) bool {
	var resource map[string]any = catalog.BindingResource(binding)
	var compared int

	for key, value := range catalog.BindingResource(discovered) {
		if key == catalog.ResourceMetaKey {
			continue
		}

		if !reflect.DeepEqual(resource[key], value) {
			return false
		}

		compared++
	}

	return compared > 0
}

// resetsDirectory is the workspace directory the collections to reset are pulled into, imported from the spec file.
const resetsDirectory string = "resets"

// markResets sets reset and drops the publication ID of each pulled collection, so the control plane runs a dataflow
// reset, as the dashboard does.
func markResets(tree *catalogTree, collections []string) error {
	for _, name := range collections {
		collection, _, _ := tree.entry(collectionsKey, name)

		if collection == nil {
			return fmt.Errorf("collection %s to reset isn't in the pulled spec", name)
		}

		delete(collection, catalog.ExpectPubIDKey)

		collection["reset"] = true
	}

	return nil
}

// withImport adds file to the imports of the spec.
func withImport(spec []byte, file string) ([]byte, error) {
	var catalogSpec map[string]any

	if err := yaml.Unmarshal(spec, &catalogSpec); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", specFile, err)
	}

	imports, _ := catalogSpec[importKey].([]any)

	catalogSpec[importKey] = append(imports, file)

	return yaml.Marshal(catalogSpec)
}

func captureFiles(name string, spec catalog.CaptureSpec) (map[string][]byte, error) {
	content, err := yaml.Marshal(map[string]any{"captures": map[string]any{name: spec}})

	if err != nil {
		return nil, fmt.Errorf("marshaling capture %s: %w", name, err)
	}

	return map[string][]byte{specFile: content}, nil
}

func materializationFiles(name string, spec catalog.MaterializationSpec) (map[string][]byte, error) {
	var catalogSpec map[string]any = map[string]any{"materializations": map[string]any{name: spec}}

	document, err := utils.YAMLDocument(catalogSpec)

	if err != nil {
		return nil, fmt.Errorf("decoding materialization %s: %w", name, err)
	}

	var dekaf map[string]any = utils.Lookup(document, "materializations", name, catalog.EndpointKey, "dekaf")

	if dekaf == nil || dekaf[catalog.ConfigKey] == nil {
		content, err := yaml.Marshal(catalogSpec)

		if err != nil {
			return nil, fmt.Errorf("marshaling materialization %s: %w", name, err)
		}

		return map[string][]byte{specFile: content}, nil
	}

	config, err := yaml.Marshal(dekaf[catalog.ConfigKey])

	if err != nil {
		return nil, fmt.Errorf("marshaling Dekaf config of %s: %w", name, err)
	}

	dekaf[catalog.ConfigKey] = dekafConfigFile

	content, err := yaml.Marshal(document)

	if err != nil {
		return nil, fmt.Errorf("marshaling materialization %s: %w", name, err)
	}

	return map[string][]byte{specFile: content, dekafConfigFile: config}, nil
}

// publish publishes files after the optional prepare step completes them, the data plane applying only to new entries.
// A refused publication ID fails as a conflict, see publicationError, whether the publication or the prepare step,
// such as the discovery of a new capture, is refused.
func (client *Client) publish(ctx context.Context, files map[string][]byte, prepare func(ctx context.Context, directory string) error) error {
	ctx, cancel := client.connectorContext(ctx, "Publishing spec", files)

	defer cancel()

	return inWorkspace(files, func(directory string) error {
		if prepare != nil {
			if err := prepare(ctx, directory); err != nil {
				return publicationError(err)
			}
		}

		_, err := client.execCommand(ctx, client.withDataPlane([]string{"catalog", "publish", "--source", filepath.Join(directory, specFile), "--auto-approve"}, "--init-data-plane")...)

		return publicationError(err)
	})
}

// publicationError marks err as catalog.ErrPublicationConflict when the control plane refused an expectPubId, keeping
// its message.
func publicationError(err error) error {
	if err != nil && strings.Contains(err.Error(), "expected publication ID") {
		return conflictError{err}
	}

	return err
}

// conflictError is a publication error matching catalog.ErrPublicationConflict.
type conflictError struct {
	error
}

func (conflictError) Is(target error) bool {
	return target == catalog.ErrPublicationConflict
}

func (err conflictError) Unwrap() error {
	return err.error
}

// addDiscovered discovers every resource of the binding-less capture, then lays the declared bindings over them.
func (client *Client) addDiscovered(ctx context.Context, directory string, name string, declared []any, identifier catalog.BindingsIdentifier) error {
	tree, err := client.discover(ctx, directory)

	if err != nil {
		return err
	}

	if err := withDeclaredBindings(tree, name, declared, identifier); err != nil {
		return err
	}

	return tree.write()
}

// addResetsStep returns the step adding the collections to reset, nil without any.
func (client *Client) addResetsStep(resets []string) func(ctx context.Context, directory string) error {
	if len(resets) == 0 {
		return nil
	}

	return func(ctx context.Context, directory string) error {
		return client.addResets(ctx, directory, resets)
	}
}

// addResets pulls the collections to reset into the resets directory of the workspace, marks them, see markResets,
// and imports them from the spec file, so they publish along with it.
func (client *Client) addResets(ctx context.Context, directory string, collections []string) error {
	if err := os.Mkdir(filepath.Join(directory, resetsDirectory), 0700); err != nil {
		return fmt.Errorf("creating the workspace of the collections to reset: %w", err)
	}

	tree, err := client.pull(ctx, filepath.Join(directory, resetsDirectory), collections...)

	if err != nil {
		return err
	}

	if err := markResets(tree, collections); err != nil {
		return err
	}

	if err := tree.write(); err != nil {
		return err
	}

	return rewriteSpec(directory, func(spec []byte) ([]byte, error) {
		return withImport(spec, resetsDirectory+"/"+specFile)
	})
}

// PublishCapture publishes the capture spec, dataflow resetting the resets collections.
func (client *Client) PublishCapture(ctx context.Context, name string, spec catalog.CaptureSpec, resets []string) error {
	files, err := captureFiles(name, spec)

	if err != nil {
		return err
	}

	return client.publish(ctx, files, client.addResetsStep(resets))
}

// PublishDiscoveredCapture publishes the capture with its discovered bindings and collections, as the dashboard does.
// Discovery runs without the declared bindings, whose collections may not exist yet.
func (client *Client) PublishDiscoveredCapture(ctx context.Context, name string, spec catalog.CaptureSpec, identifier catalog.BindingsIdentifier) error {
	document, err := utils.YAMLDocument(spec)

	if err != nil {
		return fmt.Errorf("decoding capture %s: %w", name, err)
	}

	var declared []any = catalog.Bindings(document)

	document[catalog.BindingsKey] = []any{}

	files, err := captureFiles(name, catalog.RawCapture(document))

	if err != nil {
		return err
	}

	return client.publish(ctx, files, func(ctx context.Context, directory string) error {
		return client.addDiscovered(ctx, directory, name, declared, identifier)
	})
}

// PublishMaterialization publishes the materialization spec.
func (client *Client) PublishMaterialization(ctx context.Context, name string, spec catalog.MaterializationSpec) error {
	files, err := materializationFiles(name, spec)

	if err != nil {
		return err
	}

	return client.publish(ctx, files, nil)
}
