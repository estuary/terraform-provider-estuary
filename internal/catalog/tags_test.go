package catalog_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
)

type taggedSpec struct {
	Password    string            `yaml:"password,omitempty"`
	ServerCA    string            `yaml:"ssl_server_ca" secret:"true"`
	SSLMode     *string           `yaml:"sslmode" enum:"disable,require"`
	Timeout     string            `yaml:"timeout" enum:",30s,Only Changes"`
	Credentials *struct{}         `yaml:"credentials"`
	Disabled    string            `yaml:"disabled" secret:"false"`
	Ports       int               `yaml:"ports" enum:"1,2"`
	Keys        []string          `yaml:"keys" secret:"true"`
	Labels      map[string]string `yaml:"labels"`
}

func TestIsSensitiveKey(test *testing.T) {
	expectations := map[string]bool{
		"password":        true,
		"privateKey":      true,
		"ssl_client_key":  true,
		"api_key":         true,
		"sslmode":         false,
		"aws_access_key":  false,
		"ssl_server_cert": false,
	}

	for key, want := range expectations {
		if got := catalog.IsSensitiveKey(key); got != want {
			test.Errorf("%s: IsSensitiveKey = %v, want %v", key, got, want)
		}
	}
}

func TestSecretField(test *testing.T) {
	expectations := map[string]bool{
		"Password":    true,
		"ServerCA":    true,
		"SSLMode":     false,
		"Credentials": false,
		"Disabled":    false,
		"Keys":        false,
	}

	for name, want := range expectations {
		field, _ := reflect.TypeOf(taggedSpec{}).FieldByName(name)

		if got := catalog.SecretField(field); got != want {
			test.Errorf("%s: SecretField = %v, want %v", name, got, want)
		}
	}
}

func TestEnumValues(test *testing.T) {
	expectations := map[string]struct {
		want  []string
		found bool
	}{
		"SSLMode":  {[]string{"disable", "require"}, true},
		"Timeout":  {[]string{"", "30s", "Only Changes"}, true},
		"Password": {nil, false},
	}

	for name, expected := range expectations {
		field, _ := reflect.TypeOf(taggedSpec{}).FieldByName(name)

		values, found := catalog.EnumValues(field)

		if found != expected.found || !slices.Equal(values, expected.want) {
			test.Errorf("%s: EnumValues = %q, %v, want %q, %v", name, values, found, expected.want, expected.found)
		}
	}
}

func TestValidateTags(test *testing.T) {
	expectations := map[string]bool{
		"ServerCA": true,
		"SSLMode":  true,
		"Labels":   true,
		"Ports":    false,
		"Keys":     false,
	}

	for name, valid := range expectations {
		field, _ := reflect.TypeOf(taggedSpec{}).FieldByName(name)

		if err := catalog.ValidateTags(field); (err == nil) != valid {
			test.Errorf("%s: ValidateTags = %v, want valid %v", name, err, valid)
		}
	}
}
