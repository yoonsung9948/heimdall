package transport

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yoonsung9948/heimdall/internal/types"
)

// Message is a raw JSON-RPC message. Kept as raw bytes to avoid
// repeated marshal/unmarshal as it passes through gateway layers.
type Message struct {
	Raw []byte
}

// Parsed returns message parsed into jsonrpc fields.
// On failure it returns an error message indicating failure.
func (m *Message) Parsed() (*JSONRPCMessage, error) {
	var msg JSONRPCMessage
	if err := json.Unmarshal(m.Raw, &msg); err != nil {
		return nil, fmt.Errorf("parsing JSON-RPC message: %w", err)
	}
	return &msg, nil
}

// JSONRPCMessage is the parsed representation of a raw JSON-RPC message.
type JSONRPCMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *JSONRPCError    `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object returned when a request fails.
// Code follows JSON-RPC 2.0 and MCP error code conventions.
// Data carries client-facing detail without exposing internal state.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// Transport abstracts the physical connection to one upstream MCP server.
// Implementations must be safe for concurrent use.
type Transport interface {
	// Connect establishes a connection to the upstream MCP server and
	// completes the initialize handshake. Must be called before Send.
	Connect(ctx context.Context) error

	// Send forwards a JSON-RPC message to the upstream and returns the response.
	// Blocks until a response is received or ctx is cancelled.
	Send(ctx context.Context, msg *Message) (*Message, error)

	// Capabilities returns what the upstream advertised during the initialize
	// handshake. Returns nil if Connect has not been called successfully.
	Capabilities() *Capabilities

	// Close tears down the connection gracefully. In-flight Send calls
	// may return errors after Close is called.
	Close() error
}

// Capabilities holds what an upstream MCP server advertised during the
// initialize handshake. Used by the ConnectionManager to build routing
// indexes and by the federation layer for tool selection.
type Capabilities struct {
	Tools      []types.ToolDefinition
	Resources  []types.ResourceDefinition
	Prompts    []types.PromptDefinition
	ServerInfo struct {
		Name    string
		Version string
	}
}

// Errors

// UnknownToolError is returned when no upstream is registered for the requested tool
type UnknownToolError struct {
	ToolName string
}

func (e *UnknownToolError) Error() string {
	return fmt.Sprintf("unknown tool: %q", e.ToolName)
}

// UnknownResourceError is returned when no upstream is registered for the requested resource
type UnknownResourceError struct {
	URI string
}

func (e *UnknownResourceError) Error() string {
	return fmt.Sprintf("unknown resource: %q", e.URI)
}

// UpstreamUnhealthyError is returned when the target upstream is unreachable
type UpstreamUnhealthyError struct {
	UpstreamName string
	Cause        error
}

func (e *UpstreamUnhealthyError) Error() string {
	return fmt.Sprintf("upstream %q unhealthy: %v", e.UpstreamName, e.Cause)
}

func (e *UpstreamUnhealthyError) Unwrap() error {
	return e.Cause
}
