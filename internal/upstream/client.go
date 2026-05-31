package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoonsung9948/heimdall/internal/types"
)

type UpstreamClient interface {
	Tools(ctx context.Context) ([]types.ToolDefinition, error)
	Call(ctx context.Context, params mcp.CallToolParams) (*mcp.CallToolResult, error)
}

var (
	ErrNilClientSession = errors.New("nil client session")
)

type sdkClient struct {
	session *mcp.ClientSession
}

func NewSDKClient(session *mcp.ClientSession) (*sdkClient, error) {
	if session == nil {
		return nil, ErrNilClientSession
	}
	return &sdkClient{
		session: session,
	}, nil
}

func (s sdkClient) Tools(ctx context.Context) ([]types.ToolDefinition, error) {
	var tools []types.ToolDefinition
	var cursor string
	for {
		res, err := s.session.ListTools(ctx, &mcp.ListToolsParams{
			Cursor: cursor,
		})
		if err != nil {
			return nil, fmt.Errorf("calling list tools: %w", err)
		}
		for _, tool := range res.Tools {
			inputSchema, err := json.Marshal(tool.InputSchema)
			if err != nil {
				return nil, fmt.Errorf("marshaling json: %w", err)
			}
			annotations := convertAnnotations(tool.Annotations)
			tools = append(tools, types.ToolDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: inputSchema,
				Annotations: annotations,
			})
		}
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return tools, nil
}

func convertAnnotations(ta *mcp.ToolAnnotations) *types.ToolAnnotations {
	if ta == nil {
		return nil
	}
	dh := derefBool(ta.DestructiveHint)
	oh := derefBool(ta.OpenWorldHint)
	res := &types.ToolAnnotations{
		ReadOnlyHint:    ta.ReadOnlyHint,
		DestructiveHint: dh,
		IdempotentHint:  ta.IdempotentHint,
		OpenWorldHint:   oh,
	}
	return res
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

type CallToolParams struct {
	Name string
	Args json.RawMessage
}

type CallToolResult struct {
	Content []json.RawMessage
}

func (s sdkClient) Call(ctx context.Context, params *CallToolParams) (*CallToolResult, error) {
	p := &mcp.CallToolParams{
		Name:      params.Name,
		Arguments: params.Args,
	}
	res, err := s.session.CallTool(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("calling tool %q: %w", params.Name, err)
	}
	var c []json.RawMessage

	for _, content := range res.Content {
		marshaled, err := content.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("marshaling json: %w", err)
		}
		c = append(c, marshaled)
	}

	return &CallToolResult{
		Content: c,
	}, nil
}
