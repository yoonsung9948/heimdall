package upstream

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	id, ok := r.routeMap[toolName]
	if !ok {
		return nil, "", ErrMissingTool
	}
	client, ok := r.clientMap[id]
	if !ok {
		panic(fmt.Sprintf("invariant violated: route table references unknown upstream %q", id))
	}
	return client, id, nil
}
