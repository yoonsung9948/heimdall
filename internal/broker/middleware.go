package broker

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoonsung9948/heimdall/internal/types"
)

var (
	ErrMissingBroker   = errors.New("missing broker")
	ErrMissingIdentity = errors.New("missing identity")
)

type ReceivingMiddleware struct {
	broker Broker
}

func NewReceivingMiddleware(b Broker) (ReceivingMiddleware, error) {
	if b == nil {
		return ReceivingMiddleware{}, ErrMissingBroker
	}
	return ReceivingMiddleware{
		broker: b,
	}, nil
}

func (m *ReceivingMiddleware) Handle() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "tools/list" {
				identity, ok := types.IdentityFromContext(ctx)
				if !ok {
					return nil, ErrMissingIdentity
				}
				filteredTools, err := m.broker.Filter(ctx, *identity)
				if err != nil {
					return nil, fmt.Errorf("filtering tools: %w", err)
				}
				res := &mcp.ListToolsResult{
					Tools: filteredTools,
				}
				// Bypass the SDK's built-in tools/list handler — all tools come from
				// upstream registrations. We return the identity-filtered list directly.
				return res, nil
			}
			return next(ctx, method, req)
		}
	}
}
