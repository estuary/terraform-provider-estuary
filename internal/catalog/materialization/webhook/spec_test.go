package webhook

import (
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunMaterializationSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, materialization.OnIncompatibleSchemaChanges, materialization.TargetNamingStrategies)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		address string
		headers int
		url     string
	}{
		"acmeCo/webhook/minimal":       {"http://123.4.567.89:3000/", 0, "http://123.4.567.89:3000/webhook/estuary"},
		"acmeCo/webhook/absolute-path": {"https://hooks.example.com/v1/", 1, "https://other.example.com/events"},
	}

	if len(specs) != len(expectations) {
		test.Fatalf("got %d materializations, want %d", len(specs), len(expectations))
	}

	for name, expected := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			if !found {
				test.Fatal("materialization missing from sample")
			}

			var config Config = spec.Endpoint.Connector.Config

			if config.Address != expected.address {
				test.Errorf("address = %q, want %q", config.Address, expected.address)
			}

			var headers int = 0

			if config.Headers != nil {
				headers = len(config.Headers.CustomHeaders)
			}

			if headers != expected.headers {
				test.Errorf("got %d custom headers, want %d", headers, expected.headers)
			}

			base, err := url.Parse(config.Address)

			if err != nil {
				test.Fatalf("parsing address: %v", err)
			}

			path, err := url.Parse(spec.Bindings[0].Resource.RelativePath)

			if err != nil {
				test.Fatalf("parsing relativePath: %v", err)
			}

			if resolved := base.ResolveReference(path).String(); resolved != expected.url {
				test.Errorf("resolved URL = %q, want %q", resolved, expected.url)
			}
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var spec Spec = specs["acmeCo/analytics/materialize-webhook"]

	var config Config = spec.Endpoint.Connector.Config

	expectedHeaders := []CustomHeader{
		{Name: "my-header", Value: "my-value"},
		{Name: "another-header", Value: "another-value"},
	}

	if config.Headers == nil || !slices.Equal(config.Headers.CustomHeaders, expectedHeaders) {
		test.Errorf("headers = %+v, want %+v", config.Headers, expectedHeaders)
	}

	if config.Advanced == nil || config.Advanced.FeatureFlags != "field_selection" {
		test.Errorf("advanced = %+v, want feature_flags field_selection", config.Advanced)
	}

	spectest.AssertMaterializationResources(test, spec.Bindings, []Resource{
		{RelativePath: "orders"},
		{RelativePath: "crm/customers"},
	})

	var customers Binding = spec.Bindings[1]

	if customers.Fields == nil || customers.Fields.Recommended != "1" {
		test.Fatalf("customers fields = %+v, want top-level fields recommended", customers.Fields)
	}

	if _, found := customers.Fields.Require["flow_document"]; !found {
		test.Errorf("customers fields.require = %v, want flow_document", customers.Fields.Require)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			if !strings.HasSuffix(config.Address, "/") {
				test.Errorf("%s: address %q must end in a trailing '/'", name, config.Address)
			}

			if config.Headers != nil {
				for _, header := range config.Headers.CustomHeaders {
					if header.Name == "" || header.Value == "" {
						test.Errorf("%s: header %+v needs a name and a value", name, header)
					}
				}
			}

			for _, binding := range spec.Bindings {
				if binding.Resource.RelativePath == "" {
					test.Errorf("%s: binding of %s needs a relativePath", name, binding.Source.Name)
				}
			}
		}
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"headers outside customHeaders": {
			Key: "my-header",
			Content: `
				materializations:
					acmeCo/webhook/flat-headers:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-webhook:v1
								config:
									address: https://hooks.example.com/
									headers:
										my-header: my-value
						bindings: []
			`,
		},
		"method is a trigger field": {
			Key: "method",
			Content: `
				materializations:
					acmeCo/webhook/method:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-webhook:v1
								config:
									address: https://hooks.example.com/
									method: PUT
						bindings: []
			`,
		},
	}

	spectest.AssertUnknownMaterializationKeys[Spec](test, documents)
}
