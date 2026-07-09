package transport

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RunStdio(ctx context.Context, server *mcp.Server) error {
	if ctx == nil {
		return ErrNilContext
	}
	if server == nil {
		return ErrNilMCPServer
	}
	return server.Run(ctx, &mcp.StdioTransport{})
}
