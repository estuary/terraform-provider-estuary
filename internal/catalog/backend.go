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

// Session is the backend and tenant shared by the provider's resources.
type Session struct {
	Backend Backend
	Tenant  string
}

// Backend publishes, reads, deletes and discovers catalog entries by full catalog name. ResourcePaths validates a
// materialization without publishing it and returns the resource path Estuary gives each binding, in order.
type Backend interface {
	PublishCapture(ctx context.Context, name string, spec CaptureSpec, resets []string) error
	PublishDiscoveredCapture(ctx context.Context, name string, spec CaptureSpec, identifier BindingsIdentifier) error
	PublishMaterialization(ctx context.Context, name string, spec MaterializationSpec) error
	ReadSpec(ctx context.Context, name string) (map[string]any, error)
	DeleteCatalogEntry(ctx context.Context, name string) error
	DiscoverCapture(ctx context.Context, name string, spec CaptureSpec) (map[string]map[string]any, error)
	ResourcePaths(ctx context.Context, name string, spec MaterializationSpec) ([][]string, error)
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
