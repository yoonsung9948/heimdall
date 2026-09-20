package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	ErrEmptyServerName       = errors.New("empty server name")
	ErrMissingUpstreamClient = errors.New("missing upstream client")
	ErrDuplicateServer       = errors.New("register duplicate server")
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
		routeMap:  make(map[string]string), //prefixed server names
		clientMap: make(map[string]UpstreamClient),
		toolCache: make(map[string][]*mcp.Tool),
	}
}

func (r *Registry) Register(ctx context.Context, serverName string, client UpstreamClient) error {
	if serverName == "" {
		return ErrEmptyServerName
	}
	if client == nil {
		return ErrMissingUpstreamClient
	}
	tools, err := client.Tools(ctx)
	if err != nil {
		return fmt.Errorf("fetch tools for upstream %q: %w", serverName, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clientMap[serverName]; ok {
		return ErrDuplicateServer
	}
	r.clientMap[serverName] = client
	for _, tool := range tools {
		prefixed := serverName + "." + tool.Name
		r.routeMap[prefixed] = serverName
		copied := *tool
		copied.Name = prefixed
		r.toolCache[serverName] = append(r.toolCache[serverName], &copied)
	}
	return nil
}

func (r *Registry) AllTools(ctx context.Context) []*mcp.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*mcp.Tool, 0, len(r.routeMap))
	for _, tools := range r.toolCache {
		for _, t := range tools {
			cp := *t
			out = append(out, &cp)
		}
	}
	return out
}

func (r *Registry) ClientForTool(ctx context.Context, toolName string) (UpstreamClient, string, error) {
	// returns unprefixed server name
	r.mu.RLock()
	defer r.mu.RUnlock()
	serverName, ok := r.routeMap[toolName]
	if !ok {
		return nil, "", ErrMissingTool
	}
	client, ok := r.clientMap[serverName]
	if !ok {
		panic(fmt.Sprintf("invariant violated: route table references unknown upstream %q", serverName))
	}
	return client, serverName, nil
}

func (r *Registry) ToolsPerServer(ctx context.Context) map[string][]*mcp.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make(map[string][]*mcp.Tool, len(r.toolCache))
	for serverName, tools := range r.toolCache {
		out := make([]*mcp.Tool, 0, len(tools))
		for _, t := range tools {
			cp := *t
			out = append(out, &cp)
		}
		res[serverName] = out
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
