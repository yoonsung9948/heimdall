package upstream

import (
	"context"
	"errors"
	"slices"
	"sort"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewSDKClient(t *testing.T) {
	tests := []struct {
		name    string
		session *mcp.ClientSession
		wantErr error
	}{
		{
			name:    "nil client session",
			session: nil,
			wantErr: ErrNilClientSession,
		},
		{
			name:    "non-nil client session",
			session: &mcp.ClientSession{},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewSDKClient(tt.session)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewSDKClient() error = %v, want %v", err, tt.wantErr)
			}

			if tt.wantErr != nil {
				if client != nil {
					t.Error("NewSDKClient() returned a non-nil client on failure")
				}
				return
			}

			if client == nil {
				t.Fatal("NewSDKClient() returned nil without an error")
			}
			if client.session != tt.session {
				t.Error("NewSDKClient() did not preserve the supplied session")
			}
		})
	}
}
func TestTools(t *testing.T) {
	tests := []struct {
		name    string
		tools   []string
		wantErr bool
	}{
		{name: "single tool", tools: []string{"fakeserver.tool"}},
		{name: "no tools", tools: nil},
		{name: "multiple tools", tools: []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := newTestUpstream(t, tt.tools...)
			got, err := upstream.Tools(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Tools() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			gotNames := make([]string, 0, len(got))
			for _, tool := range got {
				gotNames = append(gotNames, tool.Name)
			}
			sort.Strings(gotNames)

			wantNames := append([]string(nil), tt.tools...)
			sort.Strings(wantNames)

			if !slices.Equal(gotNames, wantNames) {
				t.Errorf("Tools() names = %v, want %v", gotNames, wantNames)
			}
		})
	}
}
func newTestUpstream(t *testing.T, toolNames ...string) *sdkClient {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "fake-upstream", Version: "0.0.1"}, nil)
	for _, name := range toolNames {
		server.AddTool(&mcp.Tool{
			Name:        name,
			InputSchema: &jsonschema.Schema{Type: "object"},
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		})
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	upstream, err := NewSDKClient(clientSession)
	if err != nil {
		t.Fatalf("new sdk client: %v", err)
	}

	return upstream
}
