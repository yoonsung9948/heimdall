package upstream

import (
	"context"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolsPerServerCopies(t *testing.T) {
	tc1 := make(map[string][]*mcp.Tool)
	tc1["server1"] = []*mcp.Tool{
		{
			Description: "fake tool",
		},
	}
	r1 := Registry{
		routeMap:  nil,
		clientMap: nil,
		toolCache: tc1,
	}
	t.Run("ToolsPerServer should return a copy", func(t *testing.T) {
		out := r1.ToolsPerServer(context.TODO())
		if reflect.ValueOf(out).Pointer() == reflect.ValueOf(tc1).Pointer() {
			t.Errorf("map instance not a copy")
		}
		for client, tools := range r1.toolCache {
			copiedTools := out[client]
			if reflect.ValueOf(copiedTools).Pointer() == reflect.ValueOf(tools).Pointer() {
				t.Errorf("tools list not a copy")
			}
		}
	})
}
