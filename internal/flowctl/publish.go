package flowctl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v3"
)

const dekafConfigFile string = "dekaf.config.yaml"
const resetsFile string = "resets.flow.yaml"

// withDeclaredBindings replaces discovered bindings with declared ones of the same resource, keeping discovered
// metadata such as the path. A declared binding naming another target takes over the discovered collection.
func withDeclaredBindings(content []byte, name string, declared []any, identifier catalog.BindingsIdentifier) ([]byte, error) {
	var catalogSpec map[string]any
	var positions map[string]int = map[string]int{}

	if err := yaml.Unmarshal(content, &catalogSpec); err != nil {
		return nil, fmt.Errorf("parsing discovered spec: %w", err)
	}

	var capture map[string]any = utils.Lookup(catalogSpec, "captures", name)

	if capture == nil {
		return nil, fmt.Errorf("discovered spec has no capture %s", name)
	}

	discovered, _ := capture["bindings"].([]any)
	collections, _ := catalogSpec["collections"].(map[string]any)

	for position, binding := range discovered {
		if key, err := bindingKey(binding, identifier); err == nil {
			positions[key] = position
		}
	}

	for _, binding := range declared {
		key, err := bindingKey(binding, identifier)

		if err != nil {
			return nil, err
		}

		position, found := positions[key]

		if !found {
			discovered = append(discovered, binding)

			continue
		}

		var target string = bindingTarget(binding)
		var previous string = bindingTarget(discovered[position])

		if meta, hasMeta := utils.Lookup(discovered[position], "resource")["_meta"]; hasMeta && utils.Lookup(binding, "resource") != nil {
			utils.Lookup(binding, "resource")["_meta"] = meta
		}

		if spec, isDiscovered := collections[previous]; isDiscovered && target != previous {
			if _, exists := collections[target]; !exists {
				collections[target] = spec
			}

			delete(collections, previous)
		}

		discovered[position] = binding
	}

	capture["bindings"] = discovered

	return yaml.Marshal(catalogSpec)
}

// bindingKey identifies a binding by its resource path, else by its target.
func bindingKey(binding any, identifier catalog.BindingsIdentifier) (string, error) {
	if identifier != nil {
		path, err := identifier.ResourcePath(utils.Lookup(binding, "resource"))

		if err != nil || path != "" {
			return "path:" + path, err
		}
	}

	return "target:" + bindingTarget(binding), nil
}

func bindingTarget(binding any) string {
	object, _ := binding.(map[string]any)
	target, _ := object["target"].(string)

	return target
}

// withResets adds the live spec of each collection with reset set, so the control plane runs a dataflow reset,
// backfilling every binding of it, as the dashboard does.
func withResets(spec []byte, pulled []byte, collections []string) ([]byte, error) {
	var catalogSpec map[string]any
	var pulledSpec struct {
		Collections map[string]map[string]any `yaml:"collections"`
	}
	var resets map[string]any = map[string]any{}

	if err := yaml.Unmarshal(spec, &catalogSpec); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", specFile, err)
	}

	if err := yaml.Unmarshal(pulled, &pulledSpec); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", resetsFile, err)
	}

	for _, name := range collections {
		collection, found := pulledSpec.Collections[name]

		if !found {
			return nil, fmt.Errorf("collection %s to reset not found", name)
		}

		delete(collection, expectPubIDKey)

		collection["reset"] = true

		resets[name] = collection
	}

	catalogSpec["collections"] = resets

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

	var dekaf map[string]any = utils.Lookup(document, "materializations", name, "endpoint", "dekaf")

	if dekaf == nil || dekaf["config"] == nil {
		content, err := yaml.Marshal(catalogSpec)

		if err != nil {
			return nil, fmt.Errorf("marshaling materialization %s: %w", name, err)
		}

		return map[string][]byte{specFile: content}, nil
	}

	config, err := yaml.Marshal(dekaf["config"])

	if err != nil {
		return nil, fmt.Errorf("marshaling Dekaf config of %s: %w", name, err)
	}

	dekaf["config"] = dekafConfigFile

	content, err := yaml.Marshal(document)

	if err != nil {
		return nil, fmt.Errorf("marshaling materialization %s: %w", name, err)
	}

	return map[string][]byte{specFile: content, dekafConfigFile: config}, nil
}

// publish publishes files after the optional prepare step completes them, the data plane applying only to new entries.
func (client *Client) publish(ctx context.Context, files map[string][]byte, prepare func(ctx context.Context, directory string) error) error {
	ctx, cancel := context.WithTimeout(ctx, connectorTimeout)

	defer cancel()

	logContext, secrets := client.maskedContext(ctx)

	tflog.Debug(logContext, "Publishing spec", map[string]any{"spec": maskedSpec(files[specFile], secrets)})

	return inWorkspace(files, func(directory string) error {
		if prepare != nil {
			if err := prepare(ctx, directory); err != nil {
				return err
			}
		}

		_, err := client.execCommand(ctx, client.withDataPlane([]string{"catalog", "publish", "--source", filepath.Join(directory, specFile), "--auto-approve"}, "--init-data-plane")...)

		return err
	})
}

// addDiscovered discovers every resource of the binding-less capture, then lays the declared bindings over them.
func (client *Client) addDiscovered(ctx context.Context, directory string, name string, declared []any, identifier catalog.BindingsIdentifier) error {
	content, err := client.runOnSpec(ctx, directory, "--source", client.withDataPlane([]string{"discover", "--flat"}, "--data-plane")...)

	if err != nil {
		return err
	}

	content, err = withDeclaredBindings(content, name, declared, identifier)

	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(directory, specFile), content, 0600)
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

// addResets pulls the collections to reset into the workspace and adds them with reset set, see withResets.
func (client *Client) addResets(ctx context.Context, directory string, collections []string) error {
	var arguments []string = []string{"catalog", "pull-specs", "--target", filepath.Join(directory, resetsFile), "--flat", "--overwrite"}

	for _, collection := range collections {
		arguments = append(arguments, flagValue("--name", collection))
	}

	if _, err := client.execCommand(ctx, arguments...); err != nil {
		return err
	}

	pulled, err := os.ReadFile(filepath.Join(directory, resetsFile))

	if err != nil {
		return fmt.Errorf("reading %s: %w", resetsFile, err)
	}

	spec, err := os.ReadFile(filepath.Join(directory, specFile))

	if err != nil {
		return fmt.Errorf("reading %s: %w", specFile, err)
	}

	content, err := withResets(spec, pulled, collections)

	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(directory, specFile), content, 0600)
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

	declared, _ := document["bindings"].([]any)

	document["bindings"] = []any{}

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
