package config

type Config struct {
	Gateway  GatewayConfig           `yaml:"gateway"`
	Identity IdentityConfig          `yaml:"identity"`
	Servers  map[string]ServerConfig `yaml:"servers"` // Upstream MCP servers, keyed by server name.
}

type GatewayConfig struct {
	Listen       string `yaml:"listen"`
	PublicURL    string `yaml:"public_url"`
	Issuer       string `yaml:"issuer"`
	PolicyFile   string `yaml:"policy_file"`
	AuditLogPath string `yaml:"audit_log_path"`
}

type IdentityConfig struct {
	Provider string                  `yaml:"provider"` // v1: static_api_keys
	Clients  map[string]ClientConfig `yaml:"clients"`
}

// ClientConfig defines a downstream client identity and its credentials.
type ClientConfig struct {
	User   string   `yaml:"user"`
	Groups []string `yaml:"groups"`
	Roles  []string `yaml:"roles"`
	Key    string   `yaml:"key"`
}

// ServerConfig defines how Heimdall connects to an upstream MCP server.
type ServerConfig struct {
	Transport TransportConfig `yaml:"transport"`
}

type TransportConfig struct {
	Type    string `yaml:"type"` // v1: http
	URL     string `yaml:"url"`
	Timeout string `yaml:"timeout"`
}
