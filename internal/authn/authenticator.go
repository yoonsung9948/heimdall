package authn

import (
	"context"
	"fmt"
	"net/http"

	"github.com/yoonsung9948/heimdall/internal/types"
)

type Authenticator interface {
	Authenticate(ctx context.Context, r *http.Request) (*types.Identity, error)
}

func NewInternalError(cause error) error {
	return &InternalError{cause: cause}
}

type InternalError struct {
	cause error
}

func (e *InternalError) Error() string {
	if e == nil {
		return "internal error: <nil>"
	}
	if e.cause != nil {
		return fmt.Sprintf("internal error: %v", e.cause)
	}
	return "internal error"
}

func (e *InternalError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}
