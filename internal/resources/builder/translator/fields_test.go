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
		if got := AttributeName(key); got != want {
			test.Errorf("AttributeName(%q) = %q, want %q", key, got, want)
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

func TestFieldsRejectsTagsOnNonStrings(test *testing.T) {
	type secretList struct {
		Keys []string `yaml:"keys" secret:"true"`
	}

	type enumNumber struct {
		Port int `yaml:"port" enum:"5432,3306"`
	}

	if _, err := fields(reflect.TypeFor[secretList]()); err == nil {
		test.Error("fields accepted a secret tag on a list")
	}

	if _, err := fields(reflect.TypeFor[enumNumber]()); err == nil {
		test.Error("fields accepted an enum tag on a number")
	}
}
