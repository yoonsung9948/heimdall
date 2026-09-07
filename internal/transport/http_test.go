package transport

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServeHTTP(t *testing.T) {
	validCtx := context.Background()
	validServer := newTestMCPServer()

	tests := []struct {
		name            string
		ctx             context.Context
		listener        func(t *testing.T) net.Listener
		server          *mcp.Server
		cfg             HTTPConfig
		wantErr         error
		wantErrContains string
	}{
		{
			name:     "nil context returns error",
			ctx:      nil,
			listener: newTestListener,
			server:   validServer,
			cfg:      HTTPConfig{},
			wantErr:  ErrNilContext,
		},
		{
			name:     "nil listener returns error",
			ctx:      validCtx,
			listener: nil,
			server:   validServer,
			cfg:      HTTPConfig{},
			wantErr:  ErrNilListener,
		},
		{
			name:     "nil MCP server returns error",
			ctx:      validCtx,
			listener: newTestListener,
			server:   nil,
			cfg:      HTTPConfig{},
			wantErr:  ErrNilMCPServer,
		},
		{
			name:     "negative max body bytes returns error",
			ctx:      validCtx,
			listener: newTestListener,
			server:   validServer,
			cfg: HTTPConfig{
				MaxBodyBytes:      -1,
				SessionTimeout:    time.Second,
				ReadHeaderTimeout: time.Second,
				IdleTimeout:       time.Second,
				ShutdownTimeout:   time.Second,
			},
			wantErrContains: "max body bytes",
		},
		{
			name:     "negative session timeout returns error",
			ctx:      validCtx,
			listener: newTestListener,
			server:   validServer,
			cfg: HTTPConfig{
				MaxBodyBytes:      1,
				SessionTimeout:    -time.Second,
				ReadHeaderTimeout: time.Second,
				IdleTimeout:       time.Second,
				ShutdownTimeout:   time.Second,
			},
			wantErrContains: "session timeout",
		},
		{
			name:     "negative read header timeout returns error",
			ctx:      validCtx,
			listener: newTestListener,
			server:   validServer,
			cfg: HTTPConfig{
				MaxBodyBytes:      1,
				SessionTimeout:    time.Second,
				ReadHeaderTimeout: -time.Second,
				IdleTimeout:       time.Second,
				ShutdownTimeout:   time.Second,
			},
			wantErrContains: "read header timeout",
		},
		{
			name:     "negative idle timeout returns error",
			ctx:      validCtx,
			listener: newTestListener,
			server:   validServer,
			cfg: HTTPConfig{
				MaxBodyBytes:      1,
				SessionTimeout:    time.Second,
				ReadHeaderTimeout: time.Second,
				IdleTimeout:       -time.Second,
				ShutdownTimeout:   time.Second,
			},
			wantErrContains: "idle timeout",
		},
		{
			name:     "negative shutdown timeout returns error",
			ctx:      validCtx,
			listener: newTestListener,
			server:   validServer,
			cfg: HTTPConfig{
				MaxBodyBytes:      1,
				SessionTimeout:    time.Second,
				ReadHeaderTimeout: time.Second,
				IdleTimeout:       time.Second,
				ShutdownTimeout:   -time.Second,
			},
			wantErrContains: "shutdown timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ln net.Listener
			if tt.listener != nil {
				ln = tt.listener(t)
				defer ln.Close()
			}

			err := ServeHTTP(tt.ctx, ln, tt.server, tt.cfg)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ServeHTTP() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if tt.wantErrContains != "" {
				if err == nil {
					t.Fatalf("ServeHTTP() error = nil, want error containing %q", tt.wantErrContains)
				}
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("ServeHTTP() error = %q, want error containing %q", err, tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("ServeHTTP() error = %v, want nil", err)
			}
		})
	}

	//happy path
	t.Run("context cancellation shuts down server cleanly", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		ln := newTestListener(t)
		defer ln.Close()

		errCh := make(chan error, 1)
		go func() {
			errCh <- ServeHTTP(ctx, ln, newTestMCPServer(), HTTPConfig{})
		}()

		cancel()

		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("ServeHTTP() error = %v, want nil", err)
			}
		case <-time.After(time.Second):
			t.Fatal("ServeHTTP() did not return after context cancellation")
		}
	})
}

func TestNewStreamableHTTP(t *testing.T) {
	validServer := newTestMCPServer()
	defaultMaxBodyBytes := int64(4 << 20)
	newValidBody := func() io.Reader {
		return strings.NewReader(`{"jsonrpc": "2.0", "method": "initialize", "id": 1, "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "test", "version": "1"}}}`)
	}
	newBodyTooLarge := func() io.Reader {
		return strings.NewReader(strings.Repeat("x", 5<<20))
	}
	tests := []struct {
		name           string
		method         string
		target         string
		body           io.Reader
		maxBodyBytes   int64
		expectedStatus int
	}{
		{
			name:   `route /mcp should exist`,
			method: http.MethodPost,
			target: "/mcp",
			body:   newValidBody(),

			expectedStatus: http.StatusOK,
		},
		{
			name:   "unknown route should error",
			method: http.MethodPost,
			target: "/hello",
			body:   newValidBody(),

			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "request body too large should error",
			method: http.MethodPost,
			target: "/mcp",
			body:   newBodyTooLarge(),

			expectedStatus: http.StatusRequestEntityTooLarge,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, tt.body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")

			w := httptest.NewRecorder()
			c, err := HTTPConfig{
				MaxBodyBytes:    defaultMaxBodyBytes,
				ShutdownTimeout: time.Second,
			}.withDefaults()
			if err != nil {
				t.Fatalf("error applying defaults: %v", err)
			}
			handler := newStreamableHTTP(validServer, c)

			handler.ServeHTTP(w, req)

			result := w.Result()

			if result.StatusCode != tt.expectedStatus {
				t.Errorf("got %d, want %d", result.StatusCode, tt.expectedStatus)
			}
		})
	}
}

func newTestMCPServer() *mcp.Server {
	return mcp.NewServer(
		&mcp.Implementation{
			Name:    "test",
			Version: "0.0.1",
		},
		nil,
	)
}

func newTestListener(t *testing.T) net.Listener {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on test address: %v", err)
	}

	return ln
}
