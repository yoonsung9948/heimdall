package authn_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/yoonsung9948/heimdall/internal/authn"
	"github.com/yoonsung9948/heimdall/internal/types"
)

func TestNewMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("nil authenticator returns error", func(t *testing.T) {
		t.Parallel()

		_, err := authn.NewMiddleware(nil, discardLogger())
		if !errors.Is(err, authn.ErrMissingAuthenticator) {
			t.Fatalf("NewMiddleware() error = %v, want %v", err, authn.ErrMissingAuthenticator)
		}
	})

	t.Run("nil logger uses default logger", func(t *testing.T) {
		t.Parallel()

		_, err := authn.NewMiddleware(&stubAuthenticator{}, nil)
		if err != nil {
			t.Fatalf("NewMiddleware() error = %v, want nil", err)
		}
	})

	t.Run("valid authenticator and valid logger should not error", func(t *testing.T) {
		t.Parallel()

		_, err := authn.NewMiddleware(&stubAuthenticator{}, slog.Default())
		if err != nil {
			t.Fatalf("NewMiddleware() error = %v, want nil", err)
		}
	})
}

func TestMiddlewareRequireAuthentication(t *testing.T) {
	t.Parallel()

	validIdentity := &types.Identity{
		User:   "alice",
		Client: "cursor-dev",
		Groups: []string{"engineering"},
	}

	tests := []struct {
		name string

		authIdentity *types.Identity
		authErr      error

		wantCode       int
		wantNextCalled bool
		wantIdentity   *types.Identity
	}{
		{
			name:           "authenticated request injects identity and calls next",
			authIdentity:   validIdentity,
			authErr:        nil,
			wantCode:       http.StatusNoContent,
			wantNextCalled: true,
			wantIdentity:   validIdentity,
		},
		{
			name:           "missing key returns unauthorized",
			authIdentity:   nil,
			authErr:        authn.ErrMissingKey,
			wantCode:       http.StatusUnauthorized,
			wantNextCalled: false,
			wantIdentity:   nil,
		},
		{
			name:           "unauthenticated request returns unauthorized",
			authIdentity:   nil,
			authErr:        authn.ErrUnauthenticated,
			wantCode:       http.StatusUnauthorized,
			wantNextCalled: false,
			wantIdentity:   nil,
		},
		{
			name:           "internal authenticator error returns internal server error",
			authIdentity:   nil,
			authErr:        authn.NewInternalError(errors.New("read key store")),
			wantCode:       http.StatusInternalServerError,
			wantNextCalled: false,
			wantIdentity:   nil,
		},
		{
			name:           "unexpected authenticator error returns internal server error",
			authIdentity:   nil,
			authErr:        errors.New("unexpected auth failure"),
			wantCode:       http.StatusInternalServerError,
			wantNextCalled: false,
			wantIdentity:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			middleware, err := authn.NewMiddleware(
				&stubAuthenticator{
					identity: tt.authIdentity,
					err:      tt.authErr,
				},
				discardLogger(),
			)
			if err != nil {
				t.Fatalf("NewMiddleware() error = %v, want nil", err)
			}

			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true

				if tt.wantIdentity == nil {
					t.Fatal("next handler called but no identity was expected")
				}

				gotIdentity, ok := types.IdentityFromContext(r.Context())
				if !ok {
					t.Fatal("identity missing from request context")
				}
				if !reflect.DeepEqual(gotIdentity, tt.wantIdentity) {
					t.Fatalf("identity = %+v, want %+v", gotIdentity, tt.wantIdentity)
				}

				w.WriteHeader(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
			rec := httptest.NewRecorder()

			middleware.RequireAuthentication(next).ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("status code = %d, want %d", rec.Code, tt.wantCode)
			}

			if nextCalled != tt.wantNextCalled {
				t.Fatalf("next called = %v, want %v", nextCalled, tt.wantNextCalled)
			}
		})
	}
}

type stubAuthenticator struct {
	identity *types.Identity
	err      error
}

func (s *stubAuthenticator) Authenticate(context.Context, *http.Request) (*types.Identity, error) {
	return s.identity, s.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
