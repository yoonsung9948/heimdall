package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoonsung9948/heimdall/internal/config"
	"github.com/yoonsung9948/heimdall/internal/types"
)

var (
	ErrEmptyClientName       = errors.New("empty client name")
	ErrMissingUpstreamClient = errors.New("missing upstream client")
	ErrDuplicateClient       = errors.New("register duplicate client")
	ErrMissingTool           = errors.New("missing tool in route map")
)

type Registry struct {
	mu        sync.RWMutex
	routeMap  map[string]string
	clientMap map[string]UpstreamClient
	toolCache map[string][]*mcp.Tool
}

type Snapshot struct {
	CapturedAt time.Time
	Tools      []ToolEntry
}

type ToolEntry struct {
	ToolName   string
	ServerName string
}

func NewRegistry() *Registry {
	return &Registry{
		routeMap:  make(map[string]string),
		clientMap: make(map[string]UpstreamClient),
		toolCache: make(map[string][]*mcp.Tool),
	}
}

func (r *Registry) Register(ctx context.Context, clientName string, client UpstreamClient) error {
	if clientName == "" {
		return ErrEmptyClientName
	}
	if client == nil {
		return ErrMissingUpstreamClient
	}
	tools, err := client.Tools(ctx)
	if err != nil {
		return fmt.Errorf("fetch tools for upstream %q: %w", clientName, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clientMap[clientName]; ok {
		return ErrDuplicateClient
	}
	r.clientMap[clientName] = client
	for _, tool := range tools {
		prefixed := clientName + "." + tool.Name
		r.routeMap[prefixed] = clientName
		copied := *tool
		copied.Name = prefixed
		r.toolCache[clientName] = append(r.toolCache[clientName], &copied)
	}
	return nil
}

func (r *Registry) AllTools(ctx context.Context) ([]*mcp.Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Concat(slices.Collect(maps.Values(r.toolCache))...), nil
}

func (r *Registry) ClientForTool(ctx context.Context, toolName string) (UpstreamClient, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	clientName, ok := r.routeMap[toolName]
	if !ok {
		return nil, "", ErrMissingTool
	}
	client, ok := r.clientMap[clientName]
	if !ok {
		panic(fmt.Sprintf("invariant violated: route table references unknown upstream %q", clientName))
	}
	return client, clientName, nil
}

func (r *Registry) ToolsPerServer(ctx context.Context) map[string][]*mcp.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make(map[string][]*mcp.Tool, len(r.toolCache))
	for client, tools := range r.toolCache {
		res[client] = append([]*mcp.Tool(nil), tools...)
	}
	return res
}

func BuildSnapshot(toolsByServer map[string][]*mcp.Tool, t time.Time) Snapshot {
	tools := make([]ToolEntry, 0, len(toolsByServer))
	for srv, tl := range toolsByServer {
		for _, tool := range tl {
			tools = append(tools, ToolEntry{
				ToolName:   tool.Name,
				ServerName: srv,
			})
		}
	}
	slices.SortFunc(tools, func(a, b ToolEntry) int {
		return strings.Compare(a.ToolName, b.ToolName)
	})
	return Snapshot{
		CapturedAt: t,
		Tools:      tools,
	}
}

func ToolsFromSnapshot(s Snapshot) map[string][]*mcp.Tool {
	res := make(map[string][]*mcp.Tool, len(s.Tools))
	for _, te := range s.Tools {
		res[te.ServerName] = append(res[te.ServerName], &mcp.Tool{
			Name: te.ToolName,
		})
	}
	return res
}

func BuildIdentityList(cfg config.Config) []types.Identity {
	res := make([]types.Identity, 0, len(cfg.Identity.Clients))
	for clientName, cc := range cfg.Identity.Clients {
		res = append(res, types.IdentityFromClient(clientName, cc))
	}
	return res
}

func WriteFile(s Snapshot, path string) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("error marshaling snapshot: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("error writing snapshot to disk: %w", err)
	}
	return nil
}

func ReadFile(path string) (Snapshot, error) {
	var s Snapshot
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("error reading snapshot file: %w", err)
	}
	err = json.Unmarshal(data, &s)
	if err != nil {
		return Snapshot{}, fmt.Errorf("error unmarshaling snapshot json: %w", err)
	}
	return s, nil
}
