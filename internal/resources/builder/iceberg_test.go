package builder

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/icebergdelta"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/icebergs3"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
	"gopkg.in/yaml.v3"
)

var icebergConnector Connector = Connector{
	TypeName:      "materialization_iceberg",
	Image:         icebergs3.Image,
	ConnectorName: icebergs3.ConnectorName,
}

var s3IcebergConnector Connector = Connector{
	TypeName:      "materialization_s3_iceberg",
	Image:         icebergdelta.Image,
	ConnectorName: icebergdelta.ConnectorName,
}

func assertMaterializationSamplesRoundTrip[C, R any](test *testing.T, connector Connector, directory string) {
	codec, err := newCodec[materializationDocument[C, R]](connector, materializationType[C, R]())

	if err != nil {
		test.Fatal(err)
	}

	files, err := filepath.Glob(filepath.Join(directory, "*.yaml"))

	if err != nil || len(files) == 0 {
		test.Fatalf("no samples in %s: %v", directory, err)
	}

	for _, file := range files {
		for name, spec := range spectest.DecodeMaterializations[MaterializationSpec[C, R]](test, file) {
			value, err := codec.Value(name, spec)

			if err != nil {
				test.Fatalf("%s %s: converting to a Terraform value: %v", file, name, err)
			}

			gotName, got, err := codec.Spec(value)

			if err != nil {
				test.Fatalf("%s %s: converting back to a spec: %v", file, name, err)
			}

			if gotName != name || !reflect.DeepEqual(got, spec) {
				test.Errorf("%s %s: round trip = %s %+v, want %+v", file, name, gotName, got, spec)
			}
		}
	}
}

func TestIcebergSamplesRoundTrip(test *testing.T) {
	assertMaterializationSamplesRoundTrip[icebergs3.Config, icebergs3.Resource](test, icebergConnector, "../../catalog/materialization/icebergs3/testdata")
}

func TestS3IcebergSamplesRoundTrip(test *testing.T) {
	assertMaterializationSamplesRoundTrip[icebergdelta.Config, icebergdelta.Resource](test, s3IcebergConnector, "../../catalog/materialization/icebergdelta/testdata")
}

func TestS3IcebergDeltaUpdatesKeepsOmission(test *testing.T) {
	var explicitFalse bool = false

	codec, err := newCodec[materializationDocument[icebergdelta.Config, icebergdelta.Resource]](s3IcebergConnector, materializationType[icebergdelta.Config, icebergdelta.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	for label, deltaUpdates := range map[string]*bool{"omitted": nil, "false": &explicitFalse} {
		var spec MaterializationSpec[icebergdelta.Config, icebergdelta.Resource] = MaterializationSpec[icebergdelta.Config, icebergdelta.Resource]{
			Endpoint: endpoint.ConnectorSpec[icebergdelta.Config]{Connector: endpoint.Connector[icebergdelta.Config]{
				Image:  icebergdelta.Image,
				Config: icebergdelta.Config{Bucket: "lake", Region: "us-east-1", Namespace: "analytics", Catalog: icebergdelta.CatalogConfig{CatalogType: icebergdelta.CatalogTypeGlue}},
			}},
			Bindings: []materialization.Binding[icebergdelta.Resource]{
				{Resource: icebergdelta.Resource{Table: "orders", DeltaUpdates: deltaUpdates}, Source: materialization.BindingSource{Name: "acmeCo/orders"}},
			},
		}

		value, err := codec.Value("acmeCo/lake", spec)

		if err != nil {
			test.Fatal(err)
		}

		_, got, err := codec.Spec(value)

		if err != nil {
			test.Fatal(err)
		}

		content, err := yaml.Marshal(got.Bindings[0].Resource)

		if err != nil {
			test.Fatal(err)
		}

		if !reflect.DeepEqual(got.Bindings[0].Resource.DeltaUpdates, deltaUpdates) || strings.Contains(string(content), "delta_updates") != (deltaUpdates != nil) {
			test.Errorf("%s delta_updates published as %q, want it kept as configured", label, content)
		}
	}
}
