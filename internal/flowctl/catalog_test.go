package flowctl

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
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
							address: mongodb+srv://cluster0.abc123.mongodb.net
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
						"address":       "mongodb+srv://cluster0.abc123.mongodb.net",
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
			"bindings": []any{},
		},
		"acmeCo/warehouse/materialize-postgres": {
			"bindings": []any{
				map[string]any{"resource": map[string]any{"table": "users"}, "source": map[string]any{"name": "acmeCo/movies/users", "notBefore": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
			},
		},
		"acmeCo/missing/source-mongodb": nil,
	}

	var directory string = test.TempDir()

	if err := os.WriteFile(filepath.Join(directory, "source-postgres.config.yaml"), []byte("address: db.example.com:5432\nuser: flow_capture\npassword_sops: ENC[AES256_GCM,data:abc,type:str]\n"), 0600); err != nil {
		test.Fatal(err)
	}

	for name, want := range expectations {
		spec, err := pulledSpec(content, name, directory)

		if err != nil {
			test.Fatalf("%s: %v", name, err)
		}

		if !reflect.DeepEqual(spec, want) {
			test.Errorf("%s: spec = %v, want %v", name, spec, want)
		}
	}

	if _, err := pulledSpec([]byte("captures: [unclosed"), "acmeCo/movies/source-mongodb", directory); err == nil {
		test.Error("pulledSpec accepted an unparseable spec")
	}

	if _, err := pulledSpec(content, "acmeCo/movies/source-postgres", test.TempDir()); err == nil {
		test.Error("pulledSpec accepted a config file that doesn't exist")
	}
}

func TestFindCatalogEntry(test *testing.T) {
	outputs := map[string]struct {
		output string
		found  bool
	}{
		"json lines with camel case names": {
			output: `
				{"catalogName":"acmeCo/pg-staging","catalogType":"materialization"}
				{"catalogName":"acmeCo/pg","catalogType":"materialization"}
			`,
			found: true,
		},
		"json lines with legacy snake case names": {
			output: `{"catalog_name":"acmeCo/pg","spec_type":"materialization"}`,
			found:  true,
		},
		"json array": {
			output: `[{"catalogName":"acmeCo/pg"}]`,
			found:  true,
		},
		"prefix match only": {
			output: `{"catalogName":"acmeCo/pg-staging"}`,
			found:  false,
		},
		"empty output": {
			output: "",
			found:  false,
		},
	}

	for name, expected := range outputs {
		test.Run(name, func(test *testing.T) {
			entry, err := findCatalogEntry([]byte(expected.output), "acmeCo/pg")

			if err != nil {
				test.Fatalf("findCatalogEntry: %v", err)
			}

			if (entry != nil) != expected.found {
				test.Errorf("entry = %+v, want found = %v", entry, expected.found)
			}

			if entry != nil && entry.CatalogName != "acmeCo/pg" {
				test.Errorf("catalog name = %q, want acmeCo/pg", entry.CatalogName)
			}
		})
	}
}

func TestFindCatalogEntryRejectsMalformedOutput(test *testing.T) {
	if _, err := findCatalogEntry([]byte("Name  Type\nacmeCo/pg  capture"), "acmeCo/pg"); err == nil {
		test.Error("error = nil, want table output to be rejected")
	}
}
