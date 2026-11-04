package flowctl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"gopkg.in/yaml.v3"
)

const dekafConfigFile string = "dekaf.config.yaml"

// withDeclaredBindings replaces discovered bindings with declared ones of the same resource, keeping discovered
// metadata. A declared binding naming another target renames the discovered collection.
func withDeclaredBindings(tree *catalogTree, name string, declared []any, identifier catalog.BindingsIdentifier) error {
	var positions map[string]int = map[string]int{}
	var capture map[string]any = tree.capture(name)

	if capture == nil {
		return fmt.Errorf("discovered spec has no capture %s", name)
	}

	var discovered []any = catalog.Bindings(capture)

	for position, binding := range discovered {
		if key, err := bindingKey(binding, identifier); err == nil {
			positions[key] = position
		}
	}

	for _, binding := range declared {
		key, err := bindingKey(binding, identifier)

		if err != nil {
			return err
		}

		position, found := positions[key]

		if !found {
			discovered = append(discovered, binding)

			continue
		}

		var target string = catalog.BindingCollection(binding)
		var previous string = catalog.BindingCollection(discovered[position])

		catalog.KeepResourceMeta(discovered[position], binding)

		if collections := tree.collections(previous); collections != nil && target != previous {
			if tree.collections(target) == nil {
				collections[target] = collections[previous]
			}

			delete(collections, previous)
		}

		discovered[position] = binding
	}

	capture[catalog.BindingsKey] = discovered

	return nil
}

// bindingKey identifies a binding by its resource path, else by its target.
func bindingKey(binding any, identifier catalog.BindingsIdentifier) (string, error) {
	if identifier != nil {
		path, err := identifier.ResourcePath(catalog.BindingResource(binding))

		if err != nil || path != "" {
			return "path:" + path, err
		}
	}

	return "target:" + catalog.BindingCollection(binding), nil
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
func (client *Client) publish(ctx context.Context, files map[string][]byte, prepare func(ctx context.Context, directory string) error) error {
	ctx, cancel := client.connectorContext(ctx, "Publishing spec", files)

	defer cancel()

	return inWorkspace(files, func(directory string) error {
		if prepare != nil {
			if err := prepare(ctx, directory); err != nil {
				return err
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
