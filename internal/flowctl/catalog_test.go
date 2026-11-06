package flowctl

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
)

func TestPulledSpec(test *testing.T) {
	var content []byte = spectest.Dedent([]byte(`
		captures:
			acmeCo/movies/source-mongodb:
				autoDiscover: { addNewBindings: true, evolveIncompatibleCollections: true }
				endpoint:
					connector:
						image: ghcr.io/estuary/source-mongodb:v3
						config:
							address: mongodb+srv://cluster0.123456789.mongodb.net
							user: flow_capture
							password_sops: ENC[AES256_GCM,data:abc,type:str]
							networkTunnel:
								sshForwarding:
									sshEndpoint: ssh://bastion.example.com
									privateKey: ENC[AES256_GCM,data:def,type:str]
							sops: { mac: <encrypted>, encrypted_suffix: _sops }
				bindings:
					- resource: { database: shop, collection: users }
						target: acmeCo/movies/users
						disable: true
			acmeCo/movies/source-postgres:
				endpoint:
					connector:
						image: ghcr.io/estuary/source-postgres:v1
						config: source-postgres.config.yaml
				bindings: []
				expectPubId: 10:a5:1e:b0:27:dc:fe:5a
		materializations:
			acmeCo/warehouse/materialize-postgres:
				bindings:
					- resource: { table: users }
						source: { name: acmeCo/movies/users, notBefore: 2026-01-01T00:00:00Z }
	`))

	expectations := map[string]map[string]any{
		"acmeCo/movies/source-mongodb": {
			"autoDiscover": map[string]any{"addNewBindings": true, "evolveIncompatibleCollections": true},
			"endpoint": map[string]any{
				"connector": map[string]any{
					"image": "ghcr.io/estuary/source-mongodb:v3",
					"config": map[string]any{
						"address":       "mongodb+srv://cluster0.123456789.mongodb.net",
						"user":          "flow_capture",
						"password_sops": "ENC[AES256_GCM,data:abc,type:str]",
						"networkTunnel": map[string]any{"sshForwarding": map[string]any{"sshEndpoint": "ssh://bastion.example.com", "privateKey": "ENC[AES256_GCM,data:def,type:str]"}},
						"sops":          map[string]any{"mac": "<encrypted>", "encrypted_suffix": "_sops"},
					},
				},
			},
			"bindings": []any{
				map[string]any{"resource": map[string]any{"database": "shop", "collection": "users"}, "target": "acmeCo/movies/users", "disable": true},
			},
		},
		"acmeCo/movies/source-postgres": {
			"endpoint": map[string]any{
				"connector": map[string]any{
					"image":  "ghcr.io/estuary/source-postgres:v1",
					"config": map[string]any{"address": "db.example.com:5432", "user": "flow_capture", "password_sops": "ENC[AES256_GCM,data:abc,type:str]"},
				},
			},
			"bindings":    []any{},
			"expectPubId": "10:a5:1e:b0:27:dc:fe:5a",
		},
		"acmeCo/warehouse/materialize-postgres": {
			"bindings": []any{
				map[string]any{"resource": map[string]any{"table": "users"}, "source": map[string]any{"name": "acmeCo/movies/users", "notBefore": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
			},
		},
	}

	var tree *catalogTree = writeTree(test, map[string]string{
		"flow.yaml":               "import: [acmeCo/movies/flow.yaml]\n",
		"acmeCo/movies/flow.yaml": string(content),
		"acmeCo/movies/source-postgres.config.yaml": "address: db.example.com:5432\nuser: flow_capture\npassword_sops: ENC[AES256_GCM,data:abc,type:str]\n",
	})

	for name, want := range expectations {
		spec, err := pulledSpec(tree, name)

		if err != nil {
			test.Fatalf("%s: %v", name, err)
		}

		if !reflect.DeepEqual(spec, want) {
			test.Errorf("%s: spec = %v, want %v", name, spec, want)
		}
	}

	var missingConfig *catalogTree = writeTree(test, map[string]string{"flow.yaml": string(content)})

	if _, err := pulledSpec(missingConfig, "acmeCo/movies/source-postgres"); err == nil {
		test.Error("pulledSpec accepted a config file that doesn't exist")
	}
}

func TestPulledSpecReadsNothingAsDeleted(test *testing.T) {
	for _, content := range []string{"", "\n", "{}", "captures: {}\nmaterializations: {}\n"} {
		spec, err := pulledSpec(writeTree(test, map[string]string{"flow.yaml": content}), "acmeCo/movies/source-mongodb")

		if err != nil || spec != nil {
			test.Errorf("%q: spec = %v, %v, want nil for a deleted entry", content, spec, err)
		}
	}

	tree, err := readCatalogTree(test.TempDir())

	if err != nil {
		test.Fatalf("readCatalogTree of a workspace without spec file: %v", err)
	}

	if spec, err := pulledSpec(tree, "acmeCo/movies/source-mongodb"); err != nil || spec != nil {
		test.Errorf("spec = %v, %v, want nil when flowctl wrote no spec file", spec, err)
	}
}

func TestPulledSpecRejectsUnexpectedContent(test *testing.T) {
	expectations := map[string]string{
		"a sibling entry":           "captures:\n  acmeCo/movies-v2/source-mongodb: { bindings: [] }\n",
		"another case":              "captures:\n  AcmeCo/movies/source-mongodb: { bindings: [] }\n",
		"a collection of the name":  "collections:\n  acmeCo/movies/source-mongodb: { key: [/id] }\n",
		"a null spec":               "captures:\n  acmeCo/movies/source-mongodb: null\n",
		"no bindings":               "captures:\n  acmeCo/movies/source-mongodb: { endpoint: {} }\n",
		"null bindings":             "captures:\n  acmeCo/movies/source-mongodb: { bindings: null }\n",
		"bindings that aren't list": "materializations:\n  acmeCo/movies/source-mongodb: { bindings: {} }\n",
	}

	for name, content := range expectations {
		if spec, err := pulledSpec(writeTree(test, map[string]string{"flow.yaml": content}), "acmeCo/movies/source-mongodb"); err == nil {
			test.Errorf("%s: pulledSpec = %v, want an error rather than a deletion or removed bindings", name, spec)
		}
	}
}

func TestInlineConfigsResolvesFromTheReferringFile(test *testing.T) {
	var tree *catalogTree = writeTree(test, map[string]string{
		"flow.yaml":                        "{}\n",
		"acmeCo/movies/flow.yaml":          "{}\n",
		"acmeCo/movies/source.config.yaml": "address: db:5432\n",
	})

	for file, reference := range map[string]string{"flow.yaml": "acmeCo/movies/source.config.yaml", "acmeCo/movies/flow.yaml": "source.config.yaml"} {
		var spec map[string]any = map[string]any{"endpoint": map[string]any{"connector": map[string]any{"config": reference}}}

		if err := tree.inlineConfigs(spec, file); err != nil {
			test.Fatalf("%s: %v", file, err)
		}

		if config := utils.Lookup(spec, "endpoint", "connector", "config"); config["address"] != "db:5432" {
			test.Errorf("%s: config = %v, want %s inlined", file, config, reference)
		}
	}
}

func TestInlineConfigsRejectsPathsOutOfTheWorkspace(test *testing.T) {
	var outside string = filepath.Join(test.TempDir(), "credentials.yaml")
	var tree *catalogTree = writeTree(test, map[string]string{"flow.yaml": "{}\n"})

	if err := os.WriteFile(outside, []byte("aws_secret_access_key: leaked\n"), 0600); err != nil {
		test.Fatal(err)
	}

	if err := os.Symlink(outside, filepath.Join(tree.directory, "link.config.yaml")); err != nil {
		test.Fatal(err)
	}

	relative, err := filepath.Rel(tree.directory, outside)

	if err != nil {
		test.Fatal(err)
	}

	for _, path := range []string{relative, outside, "link.config.yaml"} {
		var spec map[string]any = map[string]any{"endpoint": map[string]any{"connector": map[string]any{"config": path}}}

		if err := tree.inlineConfigs(spec, "flow.yaml"); err == nil {
			test.Errorf("%s: inlineConfigs read a file out of the workspace: %v", path, spec)
		}
	}
}
