package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type UpstreamClient interface {
	Tools(ctx context.Context) ([]*mcp.Tool, error)
	Call(ctx context.Context, params CallToolParams) (*CallToolResult, error)
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

func (s sdkClient) Tools(ctx context.Context) ([]*mcp.Tool, error) {
	var tools []*mcp.Tool
	var cursor string
	for {
		res, err := s.session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, fmt.Errorf("fetch tools for upstream %w", err)
		}
		tools = append(tools, res.Tools...)
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return tools, nil
}

type CallToolParams struct {
	Name string
	Args json.RawMessage
}

type CallToolResult struct {
	Content []json.RawMessage
}

func (s sdkClient) Call(ctx context.Context, params CallToolParams) (*CallToolResult, error) {
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
