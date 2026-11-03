package builder

import (
	"context"
	"sync"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// liveReader returns the live value of the entry, read once, reporting false when it can't be read.
type liveReader func() (tftypes.Value, bool)

// newLiveReader returns the liveReader of the entry in state, skipping entries not created yet.
func (entryResource *entryResource[D, S]) newLiveReader(ctx context.Context, state tftypes.Value) liveReader {
	return sync.OnceValues(func() (tftypes.Value, bool) {
		name, isKnown := known[string](blockAttribute(state, nameAttribute, tftypes.String))

		if !isKnown {
			return tftypes.Value{}, false
		}

		live, err := entryResource.readLiveValue(ctx, name)

		return live, err == nil
	})
}

// liveValue returns the Terraform value of a sealed live spec without its secrets, see withoutLiveSecrets,
// returning the paths whose credentials it redacted.
func (codec *Codec[D, S]) liveValue(name string, sealed map[string]any) (tftypes.Value, []*tftypes.AttributePath, error) {
	liveSpec, err := decode[S](liveDocument(sealed), false)

	if err != nil {
		return tftypes.Value{}, nil, err
	}

	value, err := codec.Value(name, liveSpec)

	if err != nil {
		return tftypes.Value{}, nil, err
	}

	return codec.secrets.withoutLiveSecrets(value)
}

// redactedCredentialsWarning explains the credentials redacted from the live spec of an entry.
func redactedCredentialsWarning(diagnostics *diag.Diagnostics, catalogName string, paths []*tftypes.AttributePath) {
	for _, redactedPath := range paths {
		diagnostics.AddAttributeWarning(attributePath(redactedPath), "Credentials in "+attributePath(redactedPath).String()+" of "+catalogName,
			"Estuary holds credentials in this connection string, so Terraform shows them as *** instead of storing them. "+
				"The next apply publishes the configured value, which must contain only the host and connection options. Set "+
				"the credentials in user and password instead.")
	}
}

// liveDocument returns a sealed live spec without encrypted secrets, its publication ID, control plane binding
// metadata, or the deprecated source targetNaming.
func liveDocument(sealed map[string]any) map[string]any {
	var live map[string]any = catalog.WithoutSecrets(sealed)

	delete(live, catalog.ExpectPubIDKey)

	for _, binding := range catalog.Bindings(live) {
		delete(catalog.BindingResource(binding), catalog.ResourceMetaKey)
	}

	delete(utils.Lookup(live, catalog.SourceKey), "targetNaming")

	return live
}
