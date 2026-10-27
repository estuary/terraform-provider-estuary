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

// Connector identifies a connector resource by its type name suffix, default image and catalog connector name.
type Connector struct {
	TypeName      string
	Image         string
	ConnectorName string
}

// Document is the Terraform shape of a catalog entry mapping to the catalog spec S.
type Document[D, S any] interface {
	Spec() (string, S)
	FromSpec(name string, spec S) D
}

// attributesAdjuster is implemented by documents changing their inferred attributes.
type attributesAdjuster interface {
	adjustAttributes(attributes map[string]schema.Attribute)
}

// backfillModeHolder is implemented by documents whose binding resources hold a backfill mode.
type backfillModeHolder interface {
	resourceBackfillMode() string
}

// shapeMover is implemented by documents placing fields elsewhere than their spec, moving them in both directions.
type shapeMover interface {
	toSpecShape(document map[string]any)
	toTerraformShape(document map[string]any)
}

// Codec converts a catalog entry between Terraform values and its catalog spec.
type Codec[D Document[D, S], S any] struct {
	root      *translator.Object
	schema    schema.Schema
	secrets   secrets
	valueType tftypes.Type
}

// NewCodec builds the node tree and Terraform schema from the spec YAML tags.
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

	if shards, isDynamic := attributes[shardsAttribute].(schema.DynamicAttribute); isDynamic {
		shards.Computed = true

		attributes[shardsAttribute] = shards
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
		secrets:   secrets{schema: resourceSchema},
		valueType: resourceSchema.Type().TerraformType(context.Background()),
	}, nil
}

// Schema returns the Terraform schema of the resource.
func (codec *Codec[D, S]) Schema() schema.Schema {
	return codec.schema
}

// Spec converts a Terraform value into the catalog name and spec, without the secret hashes and binding names.
func (codec *Codec[D, S]) Spec(value tftypes.Value) (string, S, error) {
	var empty S

	value, err := codec.secrets.nullHashes(value)

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

	if mover, isMover := any(*new(D)).(shapeMover); isMover {
		object, _ := converted.(map[string]any)

		mover.toSpecShape(object)
	}

	document, err := decode[D](converted, true)

	if err != nil {
		return "", empty, err
	}

	name, spec := document.Spec()

	return name, spec, nil
}

// Document converts a Terraform value into the YAML document of its catalog spec.
func (codec *Codec[D, S]) Document(value tftypes.Value) (map[string]any, error) {
	_, spec, err := codec.Spec(value)

	if err != nil {
		return nil, err
	}

	return utils.YAMLDocument(spec)
}

// decode decodes a YAML value into T, strict failing on unknown keys.
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

// Value converts a catalog name and spec into a Terraform value, naming bindings after their collection.
func (codec *Codec[D, S]) Value(name string, spec S) (tftypes.Value, error) {
	var document D

	decoded, err := utils.YAMLDocument(document.FromSpec(name, spec))

	if err != nil {
		return tftypes.Value{}, err
	}

	if mover, isMover := any(document).(shapeMover); isMover {
		mover.toTerraformShape(decoded)
	}

	value, err := codec.root.TerraformValue(decoded, codec.valueType)

	if err != nil {
		return tftypes.Value{}, err
	}

	return withBindingNames(value)
}
