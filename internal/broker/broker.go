package broker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoonsung9948/heimdall/internal/audit"
	"github.com/yoonsung9948/heimdall/internal/policy"
	"github.com/yoonsung9948/heimdall/internal/types"
	"github.com/yoonsung9948/heimdall/internal/upstream"
)

type Broker interface {
	Filter(context.Context, types.Identity) ([]*mcp.Tool, error)
	Route(context.Context, types.Identity, string, *mcp.CallToolParams) (*mcp.CallToolResult, error)
}

type broker struct {
	policyEngine *policy.Engine
	registry     *upstream.Registry
	logger       audit.AuditLogger
}

var (
	ErrMissingPolicyEngine = errors.New("missing policy engine")
	ErrMissingRegistry     = errors.New("missing registry")
	ErrUnauthorized        = errors.New("unauthorized")
)

func NewBroker(pe *policy.Engine, rg *upstream.Registry, l audit.AuditLogger) (*broker, error) {
	if pe == nil {
		return &broker{}, ErrMissingPolicyEngine
	}
	if rg == nil {
		return &broker{}, ErrMissingRegistry
	}
	return &broker{
		policyEngine: pe,
		registry:     rg,
		logger:       l,
	}, nil
}

func (b *broker) Filter(ctx context.Context, identity types.Identity) ([]*mcp.Tool, error) {
	allTools, err := b.registry.AllTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting all tools: %w", err)
	}
	var filtered []*mcp.Tool
	for _, tool := range allTools {
		req := policy.Request{
			Identity: identity,
			Action:   policy.ActionToolsList,
			Resource: policy.Resource{
				Kind: policy.ResourceKindTool,
				Name: tool.Name,
			},
		}
		decision := b.policyEngine.Authorize(req)
		if decision.Allow {
			filtered = append(filtered, tool)
		}
	}
	return filtered, nil
}

func (b *broker) Route(
	ctx context.Context,
	identity types.Identity,
	toolName string,
	params *mcp.CallToolParams,
) (*mcp.CallToolResult, error) {
	req := policy.Request{
		Identity: identity,
		Action:   policy.ActionToolsCall,
		Resource: policy.Resource{
			Kind: policy.ResourceKindTool,
			Name: toolName,
		},
	}
	decision := b.policyEngine.Authorize(req)

	client, clientName, err := b.registry.ClientForTool(ctx, toolName)

	auditEvent := audit.NewAuditEvent(req, clientName, decision, time.Now())
	b.logger.Log(auditEvent)

	if !decision.Allow {
		return nil, ErrUnauthorized
	}

	if err != nil {
		return nil, fmt.Errorf("getting client: %w", err)
	}

	originalName := strings.TrimPrefix(toolName, clientName+".")
	params.Name = originalName
	res, err := client.Call(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("calling tool %q: %w", params.Name, err)
	}
	return res, nil
}
