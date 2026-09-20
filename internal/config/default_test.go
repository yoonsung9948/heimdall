package config

import "testing"

func TestApplyDefaults_FillsEmptyGatewayFields(t *testing.T) {
	cfg := &Config{}

	applyDefaults(cfg)

	if cfg.Gateway.Listen != "127.0.0.1:9090" {
		t.Errorf("Gateway.Listen = %q, want %q", cfg.Gateway.Listen, "127.0.0.1:9090")
	}
	if cfg.Gateway.PolicyFile != "policy.yaml" {
		t.Errorf("Gateway.PolicyFile = %q, want %q", cfg.Gateway.PolicyFile, "policy.yaml")
	}
}

func TestApplyDefaults_PreservesSetGatewayFields(t *testing.T) {
	cfg := &Config{
		Gateway: GatewayConfig{
			Listen:     "0.0.0.0:8080",
			PolicyFile: "custom-policy.yaml",
		},
	}

	applyDefaults(cfg)

	if cfg.Gateway.Listen != "0.0.0.0:8080" {
		t.Errorf("Gateway.Listen = %q, want unchanged %q", cfg.Gateway.Listen, "0.0.0.0:8080")
	}
	if cfg.Gateway.PolicyFile != "custom-policy.yaml" {
		t.Errorf("Gateway.PolicyFile = %q, want unchanged %q", cfg.Gateway.PolicyFile, "custom-policy.yaml")
	}
}

func TestApplyDefaults_ServerTimeout(t *testing.T) {
	tests := []struct {
		name        string
		servers     map[string]ServerConfig
		wantTimeout map[string]string
	}{
		{
			name:        "nil servers map does not panic",
			servers:     nil,
			wantTimeout: map[string]string{},
		},
		{
			name:        "empty servers map",
			servers:     map[string]ServerConfig{},
			wantTimeout: map[string]string{},
		},
		{
			name: "empty timeout gets defaulted",
			servers: map[string]ServerConfig{
				"github": {Transport: TransportConfig{Type: "http", URL: "https://api.github.com"}},
			},
			wantTimeout: map[string]string{"github": "30s"},
		},
		{
			name: "set timeout is preserved",
			servers: map[string]ServerConfig{
				"github": {Transport: TransportConfig{Type: "http", URL: "https://api.github.com", Timeout: "5s"}},
			},
			wantTimeout: map[string]string{"github": "5s"},
		},
		{
			name: "mixed: only empty timeouts get defaulted",
			servers: map[string]ServerConfig{
				"github": {Transport: TransportConfig{Type: "http", URL: "https://api.github.com"}},
				"slack":  {Transport: TransportConfig{Type: "http", URL: "https://slack.com", Timeout: "10s"}},
			},
			wantTimeout: map[string]string{"github": "30s", "slack": "10s"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Servers: tt.servers}

			applyDefaults(cfg)

			if len(cfg.Servers) != len(tt.wantTimeout) {
				t.Fatalf("len(cfg.Servers) = %d, want %d", len(cfg.Servers), len(tt.wantTimeout))
			}
			for name, wantTimeout := range tt.wantTimeout {
				got, ok := cfg.Servers[name]
				if !ok {
					t.Errorf("server %q missing from cfg.Servers", name)
					continue
				}
				if got.Transport.Timeout != wantTimeout {
					t.Errorf("server %q: Transport.Timeout = %q, want %q", name, got.Transport.Timeout, wantTimeout)
				}
			}
		})
	}
}

func TestApplyDefaults_ServerFieldsOtherThanTimeoutAreUntouched(t *testing.T) {
	cfg := &Config{
		Servers: map[string]ServerConfig{
			"github": {Transport: TransportConfig{Type: "http", URL: "https://api.github.com"}},
		},
	}

	applyDefaults(cfg)

	got := cfg.Servers["github"].Transport
	if got.Type != "http" {
		t.Errorf("Transport.Type = %q, want unchanged %q", got.Type, "http")
	}
	if got.URL != "https://api.github.com" {
		t.Errorf("Transport.URL = %q, want unchanged %q", got.URL, "https://api.github.com")
	}
}
