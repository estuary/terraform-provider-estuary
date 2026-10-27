package builder

import (
	"context"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// liveReader returns the live value of the entry, read once, reporting false when it can't be read.
type liveReader func() (tftypes.Value, bool)

// newLiveReader returns the liveReader of the entry in state, skipping entries not created yet.
func (entryResource *entryResource[D, S]) newLiveReader(ctx context.Context, state tftypes.Value) liveReader {
	var live tftypes.Value
	var found, read bool

	return func() (tftypes.Value, bool) {
		if read {
			return live, found
		}

		read = true

		name, err := attributeValue(state, tftypes.NewAttributePath().WithAttributeName("name"))

		var text string

		if err != nil || name.As(&text) != nil || entryResource.session == nil || entryResource.session.Backend == nil {
			return live, found
		}

		sealed, err := entryResource.session.Backend.ReadSpec(ctx, entryResource.catalogName(text))

		if err != nil || sealed == nil {
			return live, found
		}

		live, err = entryResource.liveValue(text, sealed)
		found = err == nil

		return live, found
	}
}

// liveValue returns the Terraform value of a sealed live spec, without its encrypted secrets.
func (entryResource *entryResource[D, S]) liveValue(name string, sealed map[string]any) (tftypes.Value, error) {
	liveSpec, err := decode[S](liveDocument(sealed), false)

	if err != nil {
		return tftypes.Value{}, err
	}

	return entryResource.codec.Value(name, liveSpec)
}

// liveDocument returns a sealed live spec without encrypted secrets, control plane binding metadata, or the deprecated
// source targetNaming.
func liveDocument(sealed map[string]any) map[string]any {
	var live map[string]any = catalog.WithoutSecrets(sealed)

	liveBindings, _ := live["bindings"].([]any)

	for _, binding := range liveBindings {
		delete(utils.Lookup(binding, "resource"), resourceMetaKey)
	}

	delete(utils.Lookup(live, "source"), "targetNaming")

	return live
}
