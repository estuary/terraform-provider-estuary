package flowctl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/mongodb"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/dekaf"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
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

	if len(lines) != 2 || !strings.HasPrefix(lines[0], "discover --data-plane=ops/dp/private/acmeCo/example-data-plane --source ") || !strings.HasPrefix(lines[1], "catalog publish --source ") {
		test.Fatalf("calls = %q, want a discovery on the data plane then a publication", lines)
	}

	var discovered string = strings.Fields(lines[0])[3]

	if !strings.Contains(lines[1], "--source "+discovered+" ") || !strings.HasSuffix(lines[1], "--init-data-plane=ops/dp/private/acmeCo/example-data-plane") {
		test.Errorf("publication %q, want the discovered spec %s published on the data plane", lines[1], discovered)
	}
}

func TestMarkResetsMarksTheCollectionsWhereFlowctlWroteThem(test *testing.T) {
	var tree *catalogTree = writeTree(test, map[string]string{
		"flow.yaml":                 "import: [acmeCo/mysql/flow.yaml, acmeCo/postgres/flow.yaml]\n",
		"acmeCo/mysql/flow.yaml":    "collections:\n  acmeCo/mysql/sales: { key: [/id], writeSchema: sales.write.schema.yaml, expectPubId: \"0011\" }\n",
		"acmeCo/postgres/flow.yaml": "collections:\n  acmeCo/postgres/sales: { key: [/id], writeSchema: sales.write.schema.yaml }\n",
	})

	if err := markResets(tree, []string{"acmeCo/mysql/sales", "acmeCo/postgres/sales"}); err != nil {
		test.Fatalf("markResets: %v", err)
	}

	for file, name := range map[string]string{"acmeCo/mysql/flow.yaml": "acmeCo/mysql/sales", "acmeCo/postgres/flow.yaml": "acmeCo/postgres/sales"} {
		var want map[string]any = map[string]any{"key": []any{"/id"}, "writeSchema": "sales.write.schema.yaml", "reset": true}

		if got := tree.files[file]["collections"].(map[string]any)[name]; !reflect.DeepEqual(got, want) {
			test.Errorf("%s in %s = %v, want %v", name, file, got, want)
		}
	}

	if err := markResets(tree, []string{"acmeCo/missing"}); err == nil || !strings.Contains(err.Error(), "acmeCo/missing") {
		test.Errorf("markResets error = %v, want the missing collection named", err)
	}
}

func TestWithImportAddsTheFile(test *testing.T) {
	var spec []byte = []byte("import: [other.flow.yaml]\ncaptures:\n  acmeCo/source-postgres:\n    bindings: []\n")

	content, err := withImport(spec, "resets/flow.yaml")

	if err != nil {
		test.Fatalf("withImport: %v", err)
	}

	var catalogSpec map[string]any

	if err := yaml.Unmarshal(content, &catalogSpec); err != nil {
		test.Fatal(err)
	}

	if !reflect.DeepEqual(catalogSpec["import"], []any{"other.flow.yaml", "resets/flow.yaml"}) || utils.Lookup(catalogSpec, "captures", "acmeCo/source-postgres") == nil {
		test.Errorf("spec = %s, want the capture and both imports", content)
	}
}

func TestWithDeclaredBindingsReplacesTheDiscoveredOnes(test *testing.T) {
	var identifier mongodb.Spec
	var name string = "acmeCo/mongodb/source-mongodb"
	var tree *catalogTree = writeTree(test, map[string]string{
		"flow.yaml": "import: [acmeCo/mongodb/flow.yaml]\n",
		"acmeCo/mongodb/flow.yaml": `import: [shop/flow.yaml]
captures:
  acmeCo/mongodb/source-mongodb:
    bindings:
      - resource: { _meta: { path: [shop, users] }, database: shop, collection: users }
        target: acmeCo/mongodb/shop/users
      - resource: { database: shop, collection: orders }
        target: acmeCo/mongodb/shop/orders
      - resource: { database: shop, collection: events }
        target: acmeCo/mongodb/shop/events
`,
		"acmeCo/mongodb/shop/flow.yaml": `collections:
  acmeCo/mongodb/shop/users: { key: [/_id], writeSchema: users.write.schema.yaml }
  acmeCo/mongodb/shop/orders: { key: [/_id], writeSchema: orders.write.schema.yaml }
  acmeCo/mongodb/shop/events: { key: [/_id], writeSchema: events.write.schema.yaml }
`,
	})
	var declared []any = []any{
		map[string]any{"resource": map[string]any{"database": "shop", "collection": "users", "captureMode": "Change Stream Incremental"}, "target": "acmeCo/mongodb/shop/users"},
		map[string]any{"resource": map[string]any{"database": "shop", "collection": "orders"}, "target": "acmeCo/orders"},
		map[string]any{"resource": map[string]any{"database": "shop", "collection": "missing"}, "target": "acmeCo/missing"},
	}

	if err := withDeclaredBindings(tree, name, declared, identifier); err != nil {
		test.Fatalf("withDeclaredBindings: %v", err)
	}

	var targets []string
	var bindings []any = catalog.Bindings(tree.capture(name))

	for _, binding := range bindings {
		targets = append(targets, catalog.BindingCollection(binding))
	}

	if !reflect.DeepEqual(targets, []string{"acmeCo/mongodb/shop/users", "acmeCo/orders", "acmeCo/mongodb/shop/events", "acmeCo/missing"}) {
		test.Errorf("targets = %v, want the declared bindings in place of the discovered ones, and the undiscovered one appended", targets)
	}

	var users map[string]any = catalog.BindingResource(bindings[0])

	if users["captureMode"] != "Change Stream Incremental" || users["_meta"] == nil {
		test.Errorf("users resource = %v, want the declared one with the discovered metadata", users)
	}

	var collections map[string]any = tree.files["acmeCo/mongodb/shop/flow.yaml"]["collections"].(map[string]any)
	orders, renamed := collections["acmeCo/orders"].(map[string]any)
	_, previous := collections["acmeCo/mongodb/shop/orders"]

	if !renamed || previous || len(collections) != 3 || orders["writeSchema"] != "orders.write.schema.yaml" {
		test.Errorf("collections = %v, want the discovered orders collection under its declared target, in the file holding its schema", collections)
	}
}

func TestPublicationErrorMarksConflicts(test *testing.T) {
	expectations := map[string]struct {
		err      error
		conflict bool
	}{
		"expected publication ID not matched": {errors.New("stderr: expected publication ID 0000000000000000 was not matched (it's actually 117efc2f6100029f)"), true},
		"build failure":                       {errors.New("stderr: failed with status: buildFailed"), false},
	}

	for name, expected := range expectations {
		var got error = publicationError(expected.err)

		if errors.Is(got, catalog.ErrPublicationConflict) != expected.conflict || got.Error() != expected.err.Error() || !errors.Is(got, expected.err) {
			test.Errorf("%s: publicationError = %v, want the same error, conflict %v", name, got, expected.conflict)
		}
	}

	if publicationError(nil) != nil {
		test.Error("publicationError(nil) != nil")
	}
}
