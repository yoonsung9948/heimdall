package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yoonsung9948/heimdall/internal/config"
)

const validInput = `
gateway:
  listen: "127.0.0.1:9090"
  public_url: "http://localhost:9090"
  issuer: "heimdall"
  policy_file: "policy.yaml"
  audit_log_path: "audit.log"

identity:
  provider: "static_api_keys"
  clients:
    test-client:
      user: "alice"
      groups: ["engineering"]
      roles: ["developer"]
      key: "test-only-key"

servers:
  upstream:
    transport:
      type: "http"
      url: "http://localhost:8080"
      timeout: "5s"
`

func TestLoadFile_ValidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	err := os.WriteFile(path, []byte(validInput), 0600)
	if err != nil {
		t.Fatalf("write test config: %v", err)
	}

	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFile() returned nil config without an error")
	}
	if got := cfg.Identity.Clients["test-client"].User; got != "alice" {
		t.Errorf("client user = %q, want %q", got, "alice")
	}
}

func TestLoadFile_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	cfg, err := config.LoadFile(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LoadFile() error = %v, want os.ErrNotExist", err)
	}
	if cfg != nil {
		t.Error("LoadFile() returned a non-nil config for a missing file")
	}
}

func validConfig() *config.Config {
	return &config.Config{
		Gateway: config.GatewayConfig{
			Listen:       "127.0.0.1:9090",
			PublicURL:    "http://localhost:9090",
			Issuer:       "heimdall",
			PolicyFile:   "policy.yaml",
			AuditLogPath: "audit.log",
		},
		Identity: config.IdentityConfig{
			Provider: "static_api_keys",
			Clients: map[string]config.ClientConfig{
				"test-client": {
					User:   "alice",
					Groups: []string{"engineering"},
					Roles:  []string{"developer"},
					Key:    "test-only-key",
				},
			},
		},
		Servers: map[string]config.ServerConfig{
			"upstream": {
				Transport: config.TransportConfig{
					Type:    "http",
					URL:     "http://localhost:8080",
					Timeout: "5s",
				},
			},
		},
	}
}

func TestLoad(t *testing.T) {

	missingTimeout := strings.Replace(
		validInput, `      timeout: "5s"`, "", 1,
	)
	wantDefaulted := validConfig()
	s := wantDefaulted.Servers["upstream"]
	s.Transport.Timeout = "30s"
	wantDefaulted.Servers["upstream"] = s

	unknownField := strings.Replace(
		validInput, "gateway:", "gatay:", 1,
	)
	missingUser := strings.Replace(
		validInput, `      user: "alice"`, "", 1,
	)
	malformed := `gateway: [`

	tests := []struct {
		name            string
		input           string
		want            *config.Config
		wantErr         bool
		wantErrContains string
	}{
		{
			name:    "valid config preserves explicit values",
			input:   validInput,
			want:    validConfig(),
			wantErr: false,
		},
		{
			name:    "defaults apply before validation",
			input:   missingTimeout,
			want:    wantDefaulted,
			wantErr: false,
		},
		{
			name:            "unknown field gets rejected",
			input:           unknownField,
			wantErrContains: "field gatay not found",
			want:            nil,
			wantErr:         true,
		},
		{
			name:            "rejects client without user",
			input:           missingUser,
			wantErrContains: "user can't be empty",
			want:            nil,
			wantErr:         true,
		},
		{
			name:            "decoder error propagates to caller",
			input:           malformed,
			wantErrContains: "decode yaml file",
			want:            nil,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.Load(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr = %t", err, tt.wantErr)
			}
			if tt.wantErrContains != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrContains)) {
				t.Errorf("Load() error = %v, want substring %q", err, tt.wantErrContains)
			}

			if !reflect.DeepEqual(cfg, tt.want) {
				t.Errorf("Load() config = %#v, want %#v", cfg, tt.want)
			}
		})
	}
}

func TestLoad_Validate(t *testing.T) {
	validCfg := validConfig()
	missingUser := strings.Replace(
		validInput, `      user: "alice"`, "", 1,
	)
	noClients := strings.Replace(
		validInput, `  clients:
    test-client:
      user: "alice"
      groups: ["engineering"]
      roles: ["developer"]
      key: "test-only-key"
`,
		"",
		1,
	)
	emptyClients := strings.Replace(
		validInput, `  clients:
    test-client:
      user: "alice"
      groups: ["engineering"]
      roles: ["developer"]
      key: "test-only-key"
`, "  clients: {}\n", 1,
	)

	missingKey := strings.Replace(
		validInput, `      key: "test-only-key"`, "", 1,
	)
	const serversBlock = `servers:
  upstream:
    transport:
      type: "http"
      url: "http://localhost:8080"
      timeout: "5s"
`
	noServers := strings.Replace(validInput, serversBlock, "", 1)
	emptyServers := strings.Replace(validInput, serversBlock, "servers: {}\n", 1)
	malformedURL := strings.Replace(validInput, "http://localhost:8080", "http://[::1", 1)
	relativeURL := strings.Replace(validInput, "http://localhost:8080", "/api", 1)
	missingHost := strings.Replace(validInput, "http://localhost:8080", "http:///api", 1)
	missingScheme := strings.Replace(validInput, "http://localhost:8080", "//localhost:8080/api", 1)
	unsupportedTransport := strings.Replace(validInput, `type: "http"`, `type: "stdio"`, 1)
	unknownTransport := strings.Replace(validInput, `type: "http"`, `type: "grpc"`, 1)
	missingTransport := strings.Replace(validInput, `      type: "http"`, "", 1)
	tests := []struct {
		name            string
		input           string
		want            *config.Config
		wantErr         bool
		wantErrContains string
	}{
		{
			name:    "valid config succeeds",
			input:   validInput,
			want:    validCfg,
			wantErr: false,
		},
		{
			name:            "rejects missing clients",
			input:           noClients,
			wantErr:         true,
			wantErrContains: "identity.clients: at least one client is required",
		},
		{
			name:            "rejects empty clients map",
			input:           emptyClients,
			wantErr:         true,
			wantErrContains: "identity.clients: at least one client is required",
		},
		{
			name:            "rejects client without user",
			input:           missingUser,
			wantErr:         true,
			wantErrContains: `client "test-client": user can't be empty`,
		},
		{
			name:            "rejects client without API key",
			input:           missingKey,
			wantErr:         true,
			wantErrContains: `client "test-client": api key can't be empty`,
		},
		{
			name:            "rejects missing servers",
			input:           noServers,
			wantErr:         true,
			wantErrContains: "servers: at least one server is required",
		},
		{
			name:            "rejects empty servers map",
			input:           emptyServers,
			wantErr:         true,
			wantErrContains: "servers: at least one server is required",
		},
		{
			name:            "rejects malformed server URL",
			input:           malformedURL,
			wantErr:         true,
			wantErrContains: `server "upstream": parse URL`,
		},
		{
			name:            "rejects relative server URL",
			input:           relativeURL,
			wantErr:         true,
			wantErrContains: "must be an absolute URL with scheme and host",
		},
		{
			name:            "rejects server URL without host",
			input:           missingHost,
			wantErr:         true,
			wantErrContains: "must be an absolute URL with scheme and host",
		},
		{
			name:            "rejects server URL without scheme",
			input:           missingScheme,
			wantErr:         true,
			wantErrContains: "must be an absolute URL with scheme and host",
		},
		{
			name:            "rejects unsupported stdio transport",
			input:           unsupportedTransport,
			wantErr:         true,
			wantErrContains: `transport type "stdio" is not yet supported`,
		},
		{
			name:            "rejects unknown transport",
			input:           unknownTransport,
			wantErr:         true,
			wantErrContains: `invalid transport type "grpc"`,
		},
		{
			name:            "rejects missing transport type",
			input:           missingTransport,
			wantErr:         true,
			wantErrContains: `invalid transport type ""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.Load(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr = %t", err, tt.wantErr)
			}
			if tt.wantErrContains != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrContains)) {
				t.Errorf("Load() error = %v, want substring %q", err, tt.wantErrContains)
			}

			if !reflect.DeepEqual(cfg, tt.want) {
				t.Errorf("Load() config = %#v, want %#v", cfg, tt.want)
			}
		})
	}
}
