package matrix

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoonsung9948/heimdall/internal/policy"
	"github.com/yoonsung9948/heimdall/internal/types"
)

func Build(engine *policy.Engine, identities []types.Identity, tools map[string][]*mcp.Tool) []Cell {
	cells := make([]Cell, 0, len(identities)*len(tools))
	for _, identity := range identities {
		for serverName, tl := range tools {
			for _, tool := range tl {
				cells = append(cells, Evaluate(engine, identity, tool, serverName))
			}
		}
	}
	return cells
}
