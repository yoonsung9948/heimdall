package authn

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/yoonsung9948/heimdall/internal/types"
)

var ErrMissingAuthenticator = errors.New("missing authenticator")

type Middleware struct {
	authenticator Authenticator
	logger        *slog.Logger
}

func NewMiddleware(a Authenticator, logger *slog.Logger) (Middleware, error) {
	if a == nil {
		return Middleware{}, ErrMissingAuthenticator
	}
	if logger == nil {
		logger = slog.Default()
	}
	return Middleware{
		authenticator: a,
		logger:        logger,
	}, nil
}

func (m Middleware) RequireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var intErr *InternalError
		identity, err := m.authenticator.Authenticate(r.Context(), r)
		if err != nil {
			switch {
			case errors.Is(err, ErrMissingKey):
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			case errors.Is(err, ErrUnauthenticated):
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			case errors.As(err, &intErr):
				m.logger.Error(err.Error(), "err", intErr.cause)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			default:
				m.logger.Error("authentication failed unexpectedly", "err", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}
		r = r.WithContext(types.WithIdentity(r.Context(), identity))
		next.ServeHTTP(w, r)
	})
}
