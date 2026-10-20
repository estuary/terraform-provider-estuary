package translator

import (
	"reflect"
	"testing"
)

func TestAttributeName(test *testing.T) {
	for key, want := range map[string]string{
		"address":                       "address",
		"aws_role_arn":                  "aws_role_arn",
		"sshEndpoint":                   "ssh_endpoint",
		"evolveIncompatibleCollections": "evolve_incompatible_collections",
		"baseURL":                       "base_url",
		"URLPath":                       "url_path",
		"s3Bucket":                      "s3_bucket",
	} {
		if got := attributeName(key); got != want {
			test.Errorf("attributeName(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestFieldsRejectsAttributeCollisions(test *testing.T) {
	type collision struct {
		CamelCase string `yaml:"sslMode"`
		SnakeCase string `yaml:"ssl_mode"`
	}

	if _, err := fields(reflect.TypeFor[collision]()); err == nil {
		test.Error("fields accepted two keys mapped to ssl_mode")
	}
}
