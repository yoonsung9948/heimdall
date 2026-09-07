package config

func applyDefaults(cfg *Config) {
	if cfg.Gateway.Listen == "" {
		cfg.Gateway.Listen = "127.0.0.1:9090"
	}
	if cfg.Gateway.PolicyFile == "" {
		cfg.Gateway.PolicyFile = "policy.yaml"
	}
	for s, c := range cfg.Servers {
		if c.Transport.Timeout == "" {
			c.Transport.Timeout = "30s"
			cfg.Servers[s] = c
		}
	}
}
