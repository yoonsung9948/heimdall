package types

import "encoding/json"

// ToolDefinition describes a tool advertised by an upstream MCP server
// during the initialize handshake. Fields map directly to the MCP spec's
// Tool object.
type ToolDefinition struct {
	// Name is the unique identifier used in tools/call requests.
	Name string `json:"name"`

	// Description explains what the tool does. Shown to the LLM to
	// help it decide when to invoke the tool.
	Description string `json:"description"`

	// InputSchema is a JSON Schema object describing the tool's
	// accepted parameters. Kept as raw JSON to avoid repeated
	// marshal/unmarshal as it passes through gateway layers.
	InputSchema json.RawMessage `json:"inputSchema"`

	// Annotations provides hints to clients about the tool's behavior —
	// whether it is read-only, destructive, or idempotent.
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// ResourceDefinition describes a resource advertised by an upstream MCP server.
// Resources are read-only data sources identified by URI.
type ResourceDefinition struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// PromptDefinition describes a reusable prompt template advertised by
// an upstream MCP server.
type PromptDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ToolAnnotations provides behavioral hints about a tool to help clients
// decide how to present or execute it.
type ToolAnnotations struct {
	// ReadOnlyHint indicates the tool does not modify state.
	ReadOnlyHint bool `json:"readOnlyHint,omitempty"`

	// DestructiveHint indicates the tool may perform irreversible actions.
	DestructiveHint bool `json:"destructiveHint,omitempty"`

	// IdempotentHint indicates repeated calls with the same args
	// produce the same result.
	IdempotentHint bool `json:"idempotentHint,omitempty"`

	// OpenWorldHint indicates the tool may interact with external systems.
	OpenWorldHint bool `json:"openWorldHint,omitempty"`
}
