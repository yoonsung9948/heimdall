package upstream

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type UpstreamClient interface {
	Tools(ctx context.Context) ([]*mcp.Tool, error)
	Call(ctx context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error)
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

func (s sdkClient) Call(ctx context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	return s.session.CallTool(ctx, params)
}
