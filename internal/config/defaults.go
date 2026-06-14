package config

func applyDefaults(cfg *Config) {
	if cfg.Gateway.Listen == "" {
		cfg.Gateway.Listen = "127.0.0.1:9090"
	}
}
