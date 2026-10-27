package flowctl

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/mongodb"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/dekaf"
	"gopkg.in/yaml.v3"
)

func TestCaptureFilesWrapSpecUnderItsName(test *testing.T) {
	spec := capture.Spec[endpoint.ConnectorSpec[map[string]any], map[string]any]{
		Endpoint: endpoint.ConnectorSpec[map[string]any]{
			Connector: endpoint.Connector[map[string]any]{
				Image:  "ghcr.io/estuary/source-dynamodb:v1",
				Config: map[string]any{"region": "us-east-1"},
			},
		},
	}

	files, err := captureFiles("acmeCo/dynamodb/source", spec)

	if err != nil {
		test.Fatalf("captureFiles: %v", err)
	}

	var content string = string(files[specFile])

	if len(files) != 1 || !strings.Contains(content, "captures:\n    acmeCo/dynamodb/source:\n") || !strings.Contains(content, "region: us-east-1") {
		test.Errorf("files = %v, want a single spec file with the capture under its name", files)
	}
}

func TestMaterializationFilesSplitDekafConfig(test *testing.T) {
	spec := dekaf.Spec{
		Endpoint: dekaf.EndpointSpec{
			Dekaf: dekaf.Endpoint{
				Variant: dekaf.VariantTinybird,
				Config: dekaf.Config{
					Token:     "<auth-token>",
					Deletions: dekaf.DeletionModeKafka,
				},
			},
		},
		Bindings: []dekaf.Binding{
			{
				Resource: dekaf.Resource{TopicName: "orders"},
				Source: materialization.BindingSource{
					Name: "acmeCo/orders",
				},
			},
		},
	}

	files, err := materializationFiles("acmeCo/dekaf/tinybird", spec)

	if err != nil {
		test.Fatalf("materializationFiles: %v", err)
	}

	var content string = string(files[specFile])

	var config string = string(files[dekafConfigFile])

	if !strings.Contains(content, "config: "+dekafConfigFile) || strings.Contains(content, "<auth-token>") {
		test.Errorf("spec file = %s, want the Dekaf config replaced by %s", content, dekafConfigFile)
	}

	if !strings.Contains(config, "token: <auth-token>") || !strings.Contains(config, "deletions: kafka") {
		test.Errorf("config file = %s, want the Dekaf config", config)
	}
}

func TestMaterializationFilesKeepConnectorConfigInline(test *testing.T) {
	spec := materialization.Spec[endpoint.ConnectorSpec[map[string]any], map[string]any]{
		Endpoint: endpoint.ConnectorSpec[map[string]any]{
			Connector: endpoint.Connector[map[string]any]{
				Image:  "ghcr.io/estuary/materialize-snowflake:v4",
				Config: map[string]any{"host": "orgname-accountname.snowflakecomputing.com"},
			},
		},
	}

	files, err := materializationFiles("acmeCo/snowflake/materialize", spec)

	if err != nil {
		test.Fatalf("materializationFiles: %v", err)
	}

	if len(files) != 1 || !strings.Contains(string(files[specFile]), "host: orgname-accountname.snowflakecomputing.com") {
		test.Errorf("files = %v, want a single spec file with the config inline", files)
	}
}

func TestPublishDiscoveredCaptureDiscoversBeforePublishing(test *testing.T) {
	var calls string = filepath.Join(test.TempDir(), "calls")
	var client *Client = fakeFlowctl(test, "#!/bin/sh\necho \"$*\" >> \"$FLOWCTL_CALLS\"\n")

	client.dataPlane = "ops/dp/private/acmeCo/example-data-plane"

	test.Setenv("FLOWCTL_CALLS", calls)

	spec := capture.Spec[endpoint.ConnectorSpec[map[string]any], map[string]any]{
		AutoDiscover: &capture.AutoDiscover{AddNewBindings: true},
		Endpoint: endpoint.ConnectorSpec[map[string]any]{
			Connector: endpoint.Connector[map[string]any]{
				Image:  "ghcr.io/estuary/source-mongodb:v3",
				Config: map[string]any{"address": "mongodb://db.example.com"},
			},
		},
	}

	if err := client.PublishDiscoveredCapture(context.Background(), "acmeCo/mongodb/source-mongodb", spec, nil); err != nil {
		test.Fatal(err)
	}

	content, err := os.ReadFile(calls)

	if err != nil {
		test.Fatal(err)
	}

	var lines []string = strings.Split(strings.TrimSpace(string(content)), "\n")

	if len(lines) != 2 || !strings.HasPrefix(lines[0], "discover --flat --data-plane=ops/dp/private/acmeCo/example-data-plane --source ") || !strings.HasPrefix(lines[1], "catalog publish --source ") {
		test.Fatalf("calls = %q, want a discovery on the data plane then a publication", lines)
	}

	var discovered string = strings.Fields(lines[0])[4]

	if !strings.Contains(lines[1], "--source "+discovered+" ") || !strings.HasSuffix(lines[1], "--init-data-plane=ops/dp/private/acmeCo/example-data-plane") {
		test.Errorf("publication %q, want the discovered spec %s published on the data plane", lines[1], discovered)
	}
}

func TestWithResetsAddsTheCollectionsWithReset(test *testing.T) {
	var spec []byte = []byte("captures:\n  acmeCo/source-postgres:\n    bindings: []\n")
	var pulled []byte = []byte("collections:\n  acmeCo/orders:\n    key: [/id]\n    writeSchema: orders.write.schema.yaml\n    expectPubId: 0011223344556677\n  acmeCo/users:\n    key: [/id]\n")

	content, err := withResets(spec, pulled, []string{"acmeCo/orders"})

	if err != nil {
		test.Fatalf("withResets: %v", err)
	}

	var catalogSpec map[string]map[string]map[string]any

	if err := yaml.Unmarshal(content, &catalogSpec); err != nil {
		test.Fatal(err)
	}

	var want map[string]any = map[string]any{"key": []any{"/id"}, "writeSchema": "orders.write.schema.yaml", "reset": true}

	if len(catalogSpec["collections"]) != 1 || !reflect.DeepEqual(catalogSpec["collections"]["acmeCo/orders"], want) || catalogSpec["captures"]["acmeCo/source-postgres"] == nil {
		test.Errorf("spec = %s, want the capture and only the reset collection, without its publication ID", content)
	}

	if _, err := withResets(spec, pulled, []string{"acmeCo/missing"}); err == nil || !strings.Contains(err.Error(), "acmeCo/missing") {
		test.Errorf("withResets error = %v, want the missing collection named", err)
	}
}

func TestWithDeclaredBindingsReplacesTheDiscoveredOnes(test *testing.T) {
	var identifier mongodb.Spec
	var name string = "acmeCo/mongodb/source-mongodb"
	var discovered []byte = []byte(`captures:
  acmeCo/mongodb/source-mongodb:
    bindings:
      - resource: { _meta: { path: [shop, users] }, database: shop, collection: users }
        target: acmeCo/mongodb/shop/users
      - resource: { database: shop, collection: orders }
        target: acmeCo/mongodb/shop/orders
      - resource: { database: shop, collection: events }
        target: acmeCo/mongodb/shop/events
collections:
  acmeCo/mongodb/shop/users: { key: [/_id] }
  acmeCo/mongodb/shop/orders: { key: [/_id] }
  acmeCo/mongodb/shop/events: { key: [/_id] }
`)
	var declared []any = []any{
		map[string]any{"resource": map[string]any{"database": "shop", "collection": "users", "captureMode": "Change Stream Incremental"}, "target": "acmeCo/mongodb/shop/users"},
		map[string]any{"resource": map[string]any{"database": "shop", "collection": "orders"}, "target": "acmeCo/orders"},
		map[string]any{"resource": map[string]any{"database": "shop", "collection": "missing"}, "target": "acmeCo/missing"},
	}

	content, err := withDeclaredBindings(discovered, name, declared, identifier)

	if err != nil {
		test.Fatalf("withDeclaredBindings: %v", err)
	}

	var catalogSpec struct {
		Captures    map[string]map[string][]map[string]any `yaml:"captures"`
		Collections map[string]any                         `yaml:"collections"`
	}

	if err := yaml.Unmarshal(content, &catalogSpec); err != nil {
		test.Fatal(err)
	}

	var targets []string

	for _, binding := range catalogSpec.Captures[name]["bindings"] {
		targets = append(targets, binding["target"].(string))
	}

	if !reflect.DeepEqual(targets, []string{"acmeCo/mongodb/shop/users", "acmeCo/orders", "acmeCo/mongodb/shop/events", "acmeCo/missing"}) {
		test.Errorf("targets = %v, want the declared bindings in place of the discovered ones, and the undiscovered one appended", targets)
	}

	var users map[string]any = catalogSpec.Captures[name]["bindings"][0]["resource"].(map[string]any)

	if users["captureMode"] != "Change Stream Incremental" || users["_meta"] == nil {
		test.Errorf("users resource = %v, want the declared one with the discovered metadata", users)
	}

	_, renamed := catalogSpec.Collections["acmeCo/orders"]
	_, previous := catalogSpec.Collections["acmeCo/mongodb/shop/orders"]

	if !renamed || previous || len(catalogSpec.Collections) != 3 {
		test.Errorf("collections = %v, want the discovered orders collection under its declared target", catalogSpec.Collections)
	}
}
