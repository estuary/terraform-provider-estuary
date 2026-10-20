package translator

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestObjectAttributesHashWriteOnlySecrets(test *testing.T) {
	type tunnel struct {
		PrivateKey string `yaml:"privateKey"`
	}

	type config struct {
		Host     string   `yaml:"host"`
		Password string   `yaml:"password"`
		Tunnel   *tunnel  `yaml:"tunnel,omitempty"`
		Tokens   []string `yaml:"tokens,omitempty"`
	}

	object, err := NewObject(reflect.TypeFor[config]())

	if err != nil {
		test.Fatal(err)
	}

	var attributes map[string]schema.Attribute = object.Attributes()

	if hash, found := attributes["password_hash"]; !found || !hash.IsComputed() || !hash.IsSensitive() || hash.IsWriteOnly() {
		test.Errorf("password_hash = %v, want a computed sensitive hash beside the write-only password", hash)
	}

	for _, attribute := range []string{"host_hash", "tunnel_hash", "tokens_hash"} {
		if _, found := attributes[attribute]; found {
			test.Errorf("attribute %s is set, want hashes only beside write-only attributes", attribute)
		}
	}

	nested, _ := attributes["tunnel"].(schema.SingleNestedAttribute)

	if _, found := nested.Attributes["private_key_hash"]; !found {
		test.Error("tunnel has no private_key_hash, want hashes beside nested write-only attributes")
	}
}

func TestNewObjectRejectsHashCollisions(test *testing.T) {
	type collision struct {
		Password     string `yaml:"password"`
		PasswordHash string `yaml:"password_hash"`
	}

	if _, err := NewObject(reflect.TypeFor[collision]()); err == nil {
		test.Error("NewObject accepted a password_hash field beside the write-only password")
	}
}
