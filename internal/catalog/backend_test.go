package catalog_test

import (
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/amazon/aurora/mysql"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/amazon/aurora/postgres"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/amazon/dynamodb"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture/mongodb"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	materializedynamodb "github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/amazon/dynamodb"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/dekaf"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/icebergdelta"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/icebergs3"
	materializemysql "github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/mysql"
	materializepostgres "github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/postgres"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/snowflake"
)

func TestSpecsAddNewBindings(test *testing.T) {
	expectations := map[string]struct {
		spec any
		want bool
	}{
		"capture adding new bindings":     {mongodb.Spec{AutoDiscover: &capture.AutoDiscover{AddNewBindings: true}}, true},
		"capture not adding new bindings": {mongodb.Spec{AutoDiscover: &capture.AutoDiscover{}}, false},
		"capture without auto discover":   {mongodb.Spec{}, false},
	}

	for name, expected := range expectations {
		adder, isAdder := expected.spec.(catalog.NewBindingsAdder)

		if !isAdder {
			test.Errorf("%s: spec does not implement catalog.NewBindingsAdder", name)
			continue
		}

		if got := adder.AddsNewBindings(); got != expected.want {
			test.Errorf("%s: AddsNewBindings = %v, want %v", name, got, expected.want)
		}
	}
}

func TestSpecsGenerateBindings(test *testing.T) {
	var sourceCapture string = "acmeCo/movies/source-mongodb"

	expectations := map[string]struct {
		spec any
		want bool
	}{
		"capture with auto discover":           {mongodb.Spec{AutoDiscover: &capture.AutoDiscover{}}, true},
		"capture without auto discover":        {mongodb.Spec{}, false},
		"materialization with source capture":  {materializepostgres.Spec{Source: &materialization.Source{Capture: &sourceCapture}}, true},
		"materialization with null capture":    {materializepostgres.Spec{Source: &materialization.Source{TargetNaming: "noSchema"}}, false},
		"materialization without source":       {materializepostgres.Spec{}, false},
		"dekaf materialization source capture": {dekaf.Spec{Source: &materialization.Source{Capture: &sourceCapture}}, true},
	}

	for name, expected := range expectations {
		generator, isGenerator := expected.spec.(catalog.BindingsGenerator)

		if !isGenerator {
			test.Errorf("%s: spec does not implement catalog.BindingsGenerator", name)
			continue
		}

		if got := generator.GeneratesBindings(); got != expected.want {
			test.Errorf("%s: GeneratesBindings = %v, want %v", name, got, expected.want)
		}
	}
}

func TestConnectorNamesMatchImages(test *testing.T) {
	images := map[string]string{
		mongodb.ConnectorName:             mongodb.Image,
		postgres.ConnectorName:            postgres.Image,
		mysql.ConnectorName:               mysql.Image,
		dynamodb.ConnectorName:            dynamodb.Image,
		materializedynamodb.ConnectorName: materializedynamodb.Image,
		icebergs3.ConnectorName:           icebergs3.Image,
		icebergdelta.ConnectorName:        icebergdelta.Image,
		snowflake.ConnectorName:           snowflake.Image,
		materializepostgres.ConnectorName: materializepostgres.Image,
		materializemysql.ConnectorName:    materializemysql.Image,
	}

	for connectorName, image := range images {
		if !strings.HasPrefix(image, "ghcr.io/estuary/"+connectorName+":") {
			test.Errorf("connector name %s does not match image %s", connectorName, image)
		}
	}
}

func TestSpecsImplementBackendKinds(test *testing.T) {
	captures := map[string]any{
		"postgres": postgres.Spec{},
		"mysql":    mysql.Spec{},
		"dynamodb": dynamodb.Spec{},
		"mongodb":  mongodb.Spec{},
	}

	materializations := map[string]any{
		"dekaf":        dekaf.Spec{},
		"icebergs3":    icebergs3.Spec{},
		"icebergdelta": icebergdelta.Spec{},
		"snowflake":    snowflake.Spec{},
		"postgres":     materializepostgres.Spec{},
		"mysql":        materializemysql.Spec{},
		"dynamodb":     materializedynamodb.Spec{},
	}

	for name, spec := range captures {
		if _, isCapture := spec.(catalog.CaptureSpec); !isCapture {
			test.Errorf("%s: spec does not implement catalog.CaptureSpec", name)
		}

		if _, isMaterialization := spec.(catalog.MaterializationSpec); isMaterialization {
			test.Errorf("%s: capture spec implements catalog.MaterializationSpec", name)
		}
	}

	for name, spec := range materializations {
		if _, isMaterialization := spec.(catalog.MaterializationSpec); !isMaterialization {
			test.Errorf("%s: spec does not implement catalog.MaterializationSpec", name)
		}

		if _, isCapture := spec.(catalog.CaptureSpec); isCapture {
			test.Errorf("%s: materialization spec implements catalog.CaptureSpec", name)
		}
	}
}
