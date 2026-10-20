package builder

import (
	"bytes"
	"context"
	"fmt"
	"reflect"

	"github.com/estuary/terraform-provider-estuary/internal/resources/builder/translator"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"gopkg.in/yaml.v3"
)

// Connector identifies a connector resource by its Terraform type name suffix, default image and the
// connector name appended to its catalog names.
type Connector struct {
	TypeName      string
	Image         string
	ConnectorName string
}

// Document is the Terraform shape of a catalog entry, D being the document itself and S the catalog spec it maps to.
type Document[D, S any] interface {
	Spec() (string, S)
	FromSpec(name string, spec S) D
}

// attributesAdjuster is implemented by documents changing the attributes inferred from their spec types.
type attributesAdjuster interface {
	adjustAttributes(attributes map[string]schema.Attribute)
}

// Codec converts a catalog entry between its Terraform values and its catalog spec, through its document D.
type Codec[D Document[D, S], S any] struct {
	root      *translator.Object
	schema    schema.Schema
	valueType tftypes.Type
}

// NewCodec builds the node tree and Terraform schema of the document from the YAML tags of its spec types.
func NewCodec[D Document[D, S], S any](connector Connector) (*Codec[D, S], error) {
	root, err := translator.NewObject(reflect.TypeFor[D]())

	if err != nil {
		return nil, fmt.Errorf("building %s schema: %w", connector.TypeName, err)
	}

	var attributes map[string]schema.Attribute = root.Attributes()

	if _, found := attributes["name"]; !found {
		return nil, fmt.Errorf("building %s schema: the document has no name", connector.TypeName)
	}

	attributes["name"] = schema.StringAttribute{
		Description: "Estuary object name, e.g. acmeCo/orders. The tenant is prefixed and the connector name appended unless already present. " +
			"Changing the resulting catalog name replaces the catalog entry.",
		Required: true,
	}

	if adjuster, isAdjuster := any(*new(D)).(attributesAdjuster); isAdjuster {
		adjuster.adjustAttributes(attributes)
	}

	if bindings, isList := attributes[bindingsAttribute].(schema.ListNestedAttribute); isList {
		bindings.NestedObject.Attributes[bindingNameAttribute] = schema.StringAttribute{
			Description: "Collection of the binding, its target in captures and its source in materializations. " +
				"Plans always show it, identifying the changed binding.",
			Computed: true,
		}

		attributes[bindingsAttribute] = bindings
	}

	if _, found := attributes["image"]; found {
		if connector.Image == "" {
			return nil, fmt.Errorf("building %s schema: the connector has no default image", connector.TypeName)
		}

		attributes["image"] = schema.StringAttribute{
			Description: "Connector image, defaulting to " + connector.Image + ".",
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(connector.Image),
		}
	}

	var resourceSchema schema.Schema = schema.Schema{
		Attributes: attributes,
	}

	return &Codec[D, S]{
		root:      root,
		schema:    resourceSchema,
		valueType: resourceSchema.Type().TerraformType(context.Background()),
	}, nil
}

// Schema returns the Terraform schema of the resource.
func (codec *Codec[D, S]) Schema() schema.Schema {
	return codec.schema
}

// Spec converts a Terraform plan or state value into the catalog name and spec, without the secrets hash and the
// binding names, both unknown in plans until the provider sets them.
func (codec *Codec[D, S]) Spec(value tftypes.Value) (string, S, error) {
	var empty S

	value, err := codec.nullSecretsHash(value)

	if err != nil {
		return "", empty, err
	}

	value, err = nullAttributes(value, func(path *tftypes.AttributePath) bool {
		_, isName := bindingAttributeIndex(path, bindingNameAttribute)

		return isName
	})

	if err != nil {
		return "", empty, err
	}

	converted, err := codec.root.SpecValue(value)

	if err != nil {
		return "", empty, err
	}

	document, err := decode[D](converted, true)

	if err != nil {
		return "", empty, err
	}

	name, spec := document.Spec()

	return name, spec, nil
}

// LiveSpec decodes a live catalog spec, strict failing on the keys the spec doesn't model.
func (codec *Codec[D, S]) LiveSpec(live map[string]any, strict bool) (S, error) {
	return decode[S](live, strict)
}

func decode[T any](value any, strict bool) (T, error) {
	var decoded T

	content, err := yaml.Marshal(value)

	if err != nil {
		return decoded, err
	}

	var decoder *yaml.Decoder = yaml.NewDecoder(bytes.NewReader(content))

	decoder.KnownFields(strict)

	if err := decoder.Decode(&decoded); err != nil {
		return decoded, fmt.Errorf("decoding spec: %w", err)
	}

	return decoded, nil
}

// Value converts a catalog name and spec into its Terraform value, naming each binding after its collection.
func (codec *Codec[D, S]) Value(name string, spec S) (tftypes.Value, error) {
	var document D

	decoded, err := utils.YAMLDocument(document.FromSpec(name, spec))

	if err != nil {
		return tftypes.Value{}, err
	}

	value, err := codec.root.TerraformValue(decoded, codec.valueType)

	if err != nil {
		return tftypes.Value{}, err
	}

	return withBindingNames(value)
}
