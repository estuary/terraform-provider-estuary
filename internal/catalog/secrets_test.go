package catalog_test

import (
	"reflect"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

func TestWithoutSecrets(test *testing.T) {
	var sealed map[string]any = map[string]any{
		"endpoint": map[string]any{
			"connector": map[string]any{
				"image": "ghcr.io/estuary/source-mongodb:v3",
				"config": map[string]any{
					"address":       "mongodb+srv://cluster0.123456789.mongodb.net",
					"password_sops": "ENC[AES256_GCM,data:abc,type:str]",
					"networkTunnel": map[string]any{"sshForwarding": map[string]any{"sshEndpoint": "ssh://bastion.example.com", "privateKey": "ENC[AES256_GCM,data:def,type:str]"}},
					"sops":          map[string]any{"mac": "<encrypted>", "encrypted_suffix": "_sops"},
				},
			},
		},
		"bindings": []any{map[string]any{"resource": map[string]any{"collection": "users"}, "target": "acmeCo/users"}},
	}

	var want map[string]any = map[string]any{
		"endpoint": map[string]any{
			"connector": map[string]any{
				"image": "ghcr.io/estuary/source-mongodb:v3",
				"config": map[string]any{
					"address":       "mongodb+srv://cluster0.123456789.mongodb.net",
					"networkTunnel": map[string]any{"sshForwarding": map[string]any{"sshEndpoint": "ssh://bastion.example.com"}},
				},
			},
		},
		"bindings": []any{map[string]any{"resource": map[string]any{"collection": "users"}, "target": "acmeCo/users"}},
	}

	if stripped := catalog.WithoutSecrets(sealed); !reflect.DeepEqual(stripped, want) {
		test.Errorf("WithoutSecrets = %v, want %v", stripped, want)
	}

	if _, kept := sealed["endpoint"].(map[string]any)["connector"].(map[string]any)["config"].(map[string]any)["sops"]; !kept {
		test.Error("WithoutSecrets changed the sealed spec, want a stripped copy")
	}
}

func TestSecretsFingerprint(test *testing.T) {
	sealed := func(password string, mac string, host string) map[string]any {
		return map[string]any{
			"endpoint": map[string]any{
				"connector": map[string]any{
					"config": map[string]any{
						"host":          host,
						"password_sops": password,
						"headers":       []any{map[string]any{"name": "Authorization", "value": "ENC[AES256_GCM,data:hdr,type:str]"}},
						"sops":          map[string]any{"mac": mac, "encrypted_suffix": "_sops"},
					},
				},
			},
			"bindings": []any{},
		}
	}

	var stored string = catalog.SecretsFingerprint(sealed("ENC[AES256_GCM,data:abc,type:str]", "mac1", "db.example.com"))

	if stored == "" {
		test.Fatal("SecretsFingerprint is empty for an endpoint holding secrets")
	}

	if got := catalog.SecretsFingerprint(sealed("ENC[AES256_GCM,data:abc,type:str]", "mac2", "db2.example.com")); got != stored {
		test.Errorf("fingerprint = %s, want %s unchanged by sops metadata and plain fields", got, stored)
	}

	if got := catalog.SecretsFingerprint(sealed("ENC[AES256_GCM,data:rotated,type:str]", "mac1", "db.example.com")); got == stored {
		test.Error("fingerprint unchanged by a rotated secret")
	}

	if got := catalog.SecretsFingerprint(map[string]any{"endpoint": map[string]any{"connector": map[string]any{"config": map[string]any{"host": "db"}}}}); got != "" {
		test.Errorf("fingerprint = %s, want empty without secrets", got)
	}
}
