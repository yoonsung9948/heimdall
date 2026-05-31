package upstream

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

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
	toolCache map[string][]types.ToolDefinition
}

func NewRegistry() *Registry {
	return &Registry{
		routeMap:  make(map[string]string),
		clientMap: make(map[string]UpstreamClient),
		toolCache: make(map[string][]types.ToolDefinition),
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
	r.toolCache[clientName] = append([]types.ToolDefinition(nil), tools...)
	for _, tool := range tools {
		toolName := clientName + "." + tool.Name
		r.routeMap[toolName] = clientName
	}
	return nil
}

func (r *Registry) AllTools(ctx context.Context) ([]types.ToolDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Concat(slices.Collect(maps.Values(r.toolCache))...), nil
}

func (r *Registry) ClientForTool(ctx context.Context, toolName string) (UpstreamClient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.routeMap[toolName]
	if !ok {
		return nil, ErrMissingTool
	}
	client, ok := r.clientMap[id]
	if !ok {
		panic(fmt.Sprintf("invariant violated: route table references unknown upstream %q", id))
	}
	return client, nil
}
