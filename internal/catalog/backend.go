package catalog

import (
	"context"
	"errors"
	"slices"
)

// ErrPublicationConflict reports a publication refused because the live spec isn't the one it expected: another
// publication changed it, or created it when the publication creates the entry.
var ErrPublicationConflict error = errors.New("the live spec changed since it was read")

type secretsKey struct{}

// Session is the backend, tenant and data plane shared by the provider's resources. DataPlane is the one new entries
// are created in unless they set their own, empty for the default of the storage mapping.
type Session struct {
	Backend   Backend
	Tenant    string
	DataPlane string
}

// Discovery is what a capture publication takes from discovery: the specs of those Collections Estuary doesn't have
// yet, and every discovered binding with AllBindings.
type Discovery struct {
	Identifier  BindingsIdentifier
	Collections []string
	AllBindings bool
}

// Backend publishes, reads, deletes and discovers catalog entries by full catalog name. ResourcePaths validates a
// materialization without publishing it and returns the resource path Estuary gives each binding, in order.
// DataPlane names the data plane the specs a publication or validation creates are created in, and the one discovery
// runs on, empty leaving the choice to Estuary. Live entries keep their data plane, which ReadDataPlane returns by
// name, empty when the entry doesn't exist or the name can't be read.
type Backend interface {
	PublishCapture(ctx context.Context, name string, dataPlane string, spec CaptureSpec, resets []string, discovery *Discovery) error
	PublishMaterialization(ctx context.Context, name string, dataPlane string, spec MaterializationSpec) error
	ReadSpec(ctx context.Context, name string) (map[string]any, error)
	ReadDataPlane(ctx context.Context, name string) (string, error)
	DeleteCatalogEntry(ctx context.Context, name string) error
	DiscoverCapture(ctx context.Context, name string, dataPlane string, spec CaptureSpec) (map[string]map[string]any, error)
	ResourcePaths(ctx context.Context, name string, dataPlane string, spec MaterializationSpec) ([][]string, error)
}

// WithSecrets returns ctx carrying the secret values backends must mask.
func WithSecrets(ctx context.Context, secrets ...string) context.Context {
	return context.WithValue(ctx, secretsKey{}, slices.Concat(Secrets(ctx), secrets))
}

// Secrets returns the secret values ctx carries.
func Secrets(ctx context.Context) []string {
	secrets, _ := ctx.Value(secretsKey{}).([]string)

	return secrets
}
