package types

import (
	"context"

	"github.com/yoonsung9948/heimdall/internal/config"
)

type Identity struct {
	User   string
	Client string // Downstream client credential name from identity.clients.
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

func IdentityFromClient(name string, cc config.ClientConfig) Identity {
	return Identity{
		User:   cc.User,
		Client: name,
		Groups: cc.Groups,
	}
}

// BuildIdentityList returns the configured downstream client identities.
func BuildIdentityList(cfg config.Config) []Identity {
	res := make([]Identity, 0, len(cfg.Identity.Clients))
	for clientName, cc := range cfg.Identity.Clients {
		res = append(res, IdentityFromClient(clientName, cc))
	}
	return res
}
