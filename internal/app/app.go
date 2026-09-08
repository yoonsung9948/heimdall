package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoonsung9948/heimdall/internal/audit"
	"github.com/yoonsung9948/heimdall/internal/authn"
	"github.com/yoonsung9948/heimdall/internal/broker"
	"github.com/yoonsung9948/heimdall/internal/config"
	"github.com/yoonsung9948/heimdall/internal/policy"
	"github.com/yoonsung9948/heimdall/internal/transport"
	"github.com/yoonsung9948/heimdall/internal/types"
	"github.com/yoonsung9948/heimdall/internal/upstream"
)

const (
	appName    = "heimdall"
	appVersion = "0.1.0"
	bufferSize = 1024 // provisional; not derived from measured load
)

var (
	ErrUnknownClient = errors.New("unknown client")
)

func Run(ctx context.Context, cfgPath string) (err error) {
	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	policyEngine, err := policy.LoadFile(cfg.Gateway.PolicyFile)
	if err != nil {
		return fmt.Errorf("load policy: %w", err)
	}
	registry, err := BuildRegistry(ctx, cfg)
	if err != nil {
		return fmt.Errorf("build registry: %w", err)
	}

	out, err := os.OpenFile(cfg.Gateway.AuditLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}

	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing file: %w", cerr)
		}
	}()

	logger := audit.NewLogger(out, bufferSize)
	defer logger.Close()

	b, err := broker.NewBroker(policyEngine, registry, logger)
	if err != nil {
		return fmt.Errorf("construct broker: %w", err)
	}
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    appName,
			Version: appVersion,
		},
		nil,
	)
	tools, err := registry.AllTools(ctx)
	if err != nil {
		return fmt.Errorf("list all tools from registry: %w", err)
	}
	for _, t := range tools {
		server.AddTool(t, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			identity, ok := types.IdentityFromContext(ctx)
			if !ok {
				return nil, broker.ErrMissingIdentity
			}
			params := &mcp.CallToolParams{
				Name:      req.Params.Name,
				Arguments: req.Params.Arguments,
			}
			res, err := b.Route(ctx, *identity, t.Name, params)
			if err != nil {
				return nil, err
			}
			return res, nil
		})
	}
	rm, err := broker.NewReceivingMiddleware(b)
	if err != nil {
		return fmt.Errorf("construct receiving middleware: %w", err)
	}
	server.AddReceivingMiddleware(rm.Handle())
	var credentials []authn.ClientCredentials

	for name, cc := range cfg.Identity.Clients {
		id := types.IdentityFromClient(name, cc)
		credentials = append(credentials, authn.ClientCredentials{
			Name:     name,
			APIKey:   cc.Key,
			Identity: &id,
		})
	}
	authenticator, err := authn.NewAPIKeyAuthenticator(credentials)
	if err != nil {
		return fmt.Errorf("construct authenticator: %w", err)
	}
	middleware, err := authn.NewMiddleware(authenticator, nil)
	if err != nil {
		return fmt.Errorf("construct authenticator middleware: %w", err)
	}
	ln, err := net.Listen("tcp", cfg.Gateway.Listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	return transport.ServeHTTP(ctx, ln, server, transport.HTTPConfig{
		Middleware: middleware.RequireAuthentication,
	})
}

func BuildRegistry(ctx context.Context, cfg *config.Config) (*upstream.Registry, error) {
	registry := upstream.NewRegistry()
	for s, c := range cfg.Servers {
		t := c.Transport
		timeout, err := time.ParseDuration(t.Timeout)
		if err != nil {
			return nil, fmt.Errorf("parse timeout: %w", err)
		}
		mcpTransport := &mcp.StreamableClientTransport{
			Endpoint: t.URL,
			HTTPClient: &http.Client{
				Timeout: timeout,
			},
		}
		mcpClient := mcp.NewClient(
			&mcp.Implementation{
				Name:    appName,
				Version: appVersion,
			},
			nil,
		)
		session, err := mcpClient.Connect(
			ctx,
			mcpTransport,
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("client: %q: connect: %w", s, err)
		}
		client, err := upstream.NewSDKClient(session)
		if err != nil {
			return nil, fmt.Errorf("construct client: %w", err)
		}
		err = registry.Register(ctx, s, client)
		if err != nil {
			return nil, fmt.Errorf("register client: %w", err)
		}
	}
	return registry, nil
}

func LookupIdentity(name string, cfg *config.Config) (types.Identity, error) {
	cc, ok := cfg.Identity.Clients[name]
	if !ok {
		return types.Identity{}, fmt.Errorf("%q: %w", name, ErrUnknownClient)
	}
	return types.IdentityFromClient(name, cc), nil
}
