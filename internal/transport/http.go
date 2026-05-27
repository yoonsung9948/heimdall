package transport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultMaxBodyBytes      int64 = 4 << 20 // 4MiB
	defaultReadHeaderTimeout       = 10 * time.Second
	defaultIdleTimeout             = 2 * time.Minute
	defaultShutdownTimeout         = 10 * time.Second
	defaultSessionTimeout          = 30 * time.Minute
)

var (
	ErrNilContext   = errors.New("nil context")
	ErrNilListener  = errors.New("nil listener")
	ErrNilMCPServer = errors.New("nil MCP server")
)

type HTTPConfig struct {
	Logger *slog.Logger

	MaxBodyBytes      int64
	SessionTimeout    time.Duration
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

func (c HTTPConfig) withDefaults() (HTTPConfig, error) {
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.MaxBodyBytes < 0 {
		return c, errors.New("max body bytes must be non-negative")
	}
	if c.SessionTimeout < 0 {
		return c, errors.New("session timeout must be non-negative")
	}
	if c.ReadHeaderTimeout < 0 {
		return c, errors.New("read header timeout must be non-negative")
	}
	if c.IdleTimeout < 0 {
		return c, errors.New("idle timeout must be non-negative")
	}
	if c.ShutdownTimeout < 0 {
		return c, errors.New("shutdown timeout must be positive")
	}
	if c.MaxBodyBytes == 0 {
		c.MaxBodyBytes = defaultMaxBodyBytes
	}
	if c.SessionTimeout == 0 {
		c.SessionTimeout = defaultSessionTimeout
	}
	if c.ReadHeaderTimeout == 0 {
		c.ReadHeaderTimeout = defaultReadHeaderTimeout
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = defaultIdleTimeout
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = defaultShutdownTimeout
	}
	return c, nil
}

func newStreamableHTTP(server *mcp.Server, cfg HTTPConfig) http.Handler {

	mux := http.NewServeMux()

	streamHandler := mcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcp.Server {
			return server
		}, &mcp.StreamableHTTPOptions{
			Logger:         cfg.Logger,
			SessionTimeout: cfg.SessionTimeout,
		},
	)

	mux.Handle("/mcp", recovery(maxBytes(streamHandler, cfg.MaxBodyBytes)))

	return mux
}

func ServeHTTP(ctx context.Context, ln net.Listener, server *mcp.Server, cfg HTTPConfig) error {
	if ctx == nil {
		return ErrNilContext
	}
	if ln == nil {
		return ErrNilListener
	}
	if server == nil {
		return ErrNilMCPServer
	}

	cfg, err := cfg.withDefaults()
	if err != nil {
		return fmt.Errorf("validate HTTP config: %w", err)
	}

	handler := newStreamableHTTP(server, cfg)
	httpServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		err := httpServer.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()

	select {
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil

	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			_ = httpServer.Close()
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}

		if err := <-serveErr; err != nil {
			return fmt.Errorf("serve HTTP after shutdown: %w", err)
		}

		return nil
	}
}

func maxBytes(next http.Handler, limit int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > limit {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}

		next.ServeHTTP(w, r)
	})
}

func recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "err", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

var NewStreamableHTTPForTest = newStreamableHTTP
