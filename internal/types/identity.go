package types

import "context"

type Identity struct {
	User   string
	Client string
	Groups []string
}

type contextKey string

const identityKey contextKey = "identity"

// WithIdentity attaches an identity to the input context
// and returns the attached context
func WithIdentity(ctx context.Context, identity *Identity) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}

// IdentityFromContext returns an Identity from an input context
func IdentityFromContext(ctx context.Context) (*Identity, bool) {
	identity, ok := ctx.Value(identityKey).(*Identity)
	return identity, ok
}
