package matrix

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoonsung9948/heimdall/internal/policy"
	"github.com/yoonsung9948/heimdall/internal/types"
)

type Cell struct {
	Identity    types.Identity
	Tool        *mcp.Tool
	ServerName  string
	Explanation policy.Explanation
}

func Evaluate(engine *policy.Engine, identity types.Identity, tool *mcp.Tool, serverName string) Cell {
	req := policy.Request{
		Identity: identity,
		Action:   policy.ActionToolsCall,
		Resource: policy.Resource{
			Kind: policy.ResourceKindTool,
			Name: tool.Name,
		},
	}
	return Cell{
		Identity:    identity,
		Tool:        tool,
		ServerName:  serverName,
		Explanation: engine.Explain(req),
	}
}
