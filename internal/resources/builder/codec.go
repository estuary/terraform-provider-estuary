package builder

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder/translator"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

// Codec converts a catalog entry between Terraform values and its catalog spec.
type Codec[D Document[D, S], S any] struct {
	catalogType catalogType
	root        *translator.Object
	schema      schema.Schema
	secrets     secrets
	valueType   tftypes.Type
}

// nameDescription describes the name attribute of the resources and data sources of a catalog type.
func (catalogType catalogType) nameDescription() string {
	return "Catalog name of the " + catalogType.name + " (e.g., shop). The provider adds the tenant in front and the " +
		"connector name at the end, unless they're already there."
}

// imageTagPattern matches an OCI image tag.
const imageTagPattern string = `[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}`

// repository returns the default image without its tag, reporting false when it has none.
func (connector Connector) repository() (string, bool) {
	var separator int = strings.LastIndex(connector.Image, ":")

	if separator <= 0 || strings.Contains(connector.Image[separator:], "/") {
		return "", false
	}

	return connector.Image[:separator], true
}

// typeName returns the Terraform type name of the connector under the provider type name.
func (connector Connector) typeName(providerTypeName string) string {
	return providerTypeName + "_" + connector.TypeName
}

// catalogName returns the full catalog name of name, with the session tenant, empty until configured, and the
// connector name.
func (connector Connector) catalogName(session *catalog.Session, name string) string {
	var tenant string

	if session != nil {
		tenant = session.Tenant
	}

	return catalog.CatalogName(tenant, name, connector.ConnectorName)
}

// newCodec builds the node tree and Terraform schema from the spec YAML tags.
func newCodec[D Document[D, S], S any](connector Connector, catalogType catalogType) (*Codec[D, S], error) {
	root, err := translator.NewObject(reflect.TypeFor[D]())

	if err != nil {
		return nil, fmt.Errorf("building %s schema: %w", connector.TypeName, err)
	}

	var attributes map[string]schema.Attribute = root.Attributes()

	if _, found := attributes[nameAttribute]; !found {
		return nil, fmt.Errorf("building %s schema: the document has no name", connector.TypeName)
	}

	attributes[nameAttribute] = schema.StringAttribute{
		Description: catalogType.nameDescription() + " Changing the resulting catalog name replaces the " + catalogType.name + ".",
		Required:    true,
	}

	if catalogType.adjustAttributes != nil {
		catalogType.adjustAttributes(attributes)
	}

	if bindings, isList := attributes[bindingsAttribute].(schema.ListNestedAttribute); isList {
		bindings.NestedObject.Attributes[bindingNameAttribute] = schema.StringAttribute{
			Description: "Collection of the binding, its target in captures and its source in materializations. " +
				"Plans always show it, identifying the changed binding.",
			Computed: true,
		}

		attributes[bindingsAttribute] = bindings

		if catalogType.listsLiveBindings {
			attributes[liveBindingsAttribute] = liveBindingsSchema(bindings)
		}
	}

	if shards, isDynamic := attributes[shardsAttribute].(schema.DynamicAttribute); isDynamic {
		shards.Computed = true

		attributes[shardsAttribute] = shards
	}

	if _, found := attributes[imageAttribute]; found {
		repository, isTagged := connector.repository()

		if !isTagged {
			return nil, fmt.Errorf("building %s schema: the connector has no default image with a tag", connector.TypeName)
		}

		attributes[imageAttribute] = schema.StringAttribute{
			Description: "Connector image, defaulting to " + connector.Image + ". Only the tag can change, e.g. " + repository + ":dev.",
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(connector.Image),
			Validators: []validator.String{
				stringvalidator.RegexMatches(regexp.MustCompile(`^`+regexp.QuoteMeta(repository)+`:`+imageTagPattern+`$`),
					"must be "+repository+" with a tag, such as "+connector.Image),
			},
		}
	}

	var resourceSchema schema.Schema = schema.Schema{
		Attributes: attributes,
	}

	return &Codec[D, S]{
		catalogType: catalogType,
		root:        root,
		schema:      resourceSchema,
		secrets:     secrets{schema: resourceSchema},
		valueType:   resourceSchema.Type().TerraformType(context.Background()),
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

	if codec.catalogType.toSpecShape != nil {
		object, _ := converted.(map[string]any)

		codec.catalogType.toSpecShape(object)
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
	decoded, err := utils.ConvertYAML[T](value, strict)

	if err != nil {
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

	if codec.catalogType.toTerraformShape != nil {
		codec.catalogType.toTerraformShape(decoded)
	}

	value, err := codec.root.TerraformValue(decoded, codec.valueType)

	if err != nil {
		return tftypes.Value{}, err
	}

	return withBindingNames(value)
}
