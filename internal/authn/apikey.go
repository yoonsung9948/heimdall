package authn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yoonsung9948/heimdall/internal/types"
)

var (
	ErrMissingKey        = errors.New("missing API key")
	ErrUnauthenticated   = errors.New("client unauthenticated")
	ErrNoCredentials     = errors.New("no credentials given")
	ErrInvalidCredential = errors.New("invalid credential")
)

type ClientCredentials struct {
	APIKey   string
	Name     string
	Identity *types.Identity
}

type APIKeyAuthenticator struct {
	keyStore map[string]*types.Identity
}

func NewAPIKeyAuthenticator(credentials []ClientCredentials) (APIKeyAuthenticator, error) {
	if len(credentials) == 0 {
		return APIKeyAuthenticator{}, ErrNoCredentials
	}
	keyStore := make(map[string]*types.Identity, len(credentials))
	for _, c := range credentials {
		if c.Name == "" {
			return APIKeyAuthenticator{}, fmt.Errorf("invalid credential, no name: %w", ErrInvalidCredential)
		}
		if c.APIKey == "" || c.Identity == nil {
			return APIKeyAuthenticator{}, fmt.Errorf("invalid credential for client %q: %w", c.Name, ErrInvalidCredential)
		}
		hash := sha256.Sum256([]byte(c.APIKey))
		stringHash := hex.EncodeToString(hash[:])
		keyStore[stringHash] = c.Identity
	}
	return APIKeyAuthenticator{
		keyStore: keyStore,
	}, nil
}

func (a APIKeyAuthenticator) Authenticate(
	ctx context.Context,
	r *http.Request,
) (*types.Identity, error) {
	key, err := getAPIKey(r)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(key))
	stringHash := hex.EncodeToString(hash[:])
	identity, ok := a.keyStore[stringHash]
	if !ok {
		return nil, ErrUnauthenticated
	}
	return identity, nil
}

func getAPIKey(r *http.Request) (string, error) {
	key := r.Header.Get("X-API-KEY")
	if key == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			key = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if key == "" {
		return "", ErrMissingKey
	}
	return key, nil
}
