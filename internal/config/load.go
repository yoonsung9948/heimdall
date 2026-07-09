package config

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open yaml file: %w", err)
	}
	defer f.Close()
	cfg, err := Load(f)
	if err != nil {
		return nil, fmt.Errorf("load yaml file: %w", err)
	}
	return cfg, nil
}

func Load(r io.Reader) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode yaml file: %w", err)
	}
	applyDefaults(&cfg)
	if err := validateFields(cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func validateFields(cfg Config) error {
	idConfig := cfg.Identity
	serverCfgs := cfg.Servers

	if len(idConfig.Clients) == 0 {
		return errors.New("identity.clients: at least one client is required")
	}
	for name, clientCfg := range idConfig.Clients {
		if clientCfg.User == "" {
			return fmt.Errorf("client %q: user can't be empty", name)
		}
		if clientCfg.Key == "" {
			return fmt.Errorf("client %q: api key can't be empty", name)
		}
	}

	if len(serverCfgs) == 0 {
		return errors.New("servers: at least one server is required")
	}
	for srvName, srvConfig := range serverCfgs {
		switch srvConfig.Transport.Type {
		case "http":
			u, err := url.Parse(srvConfig.Transport.URL)
			if err != nil {
				return fmt.Errorf("server %q: parse URL: %w", srvName, err)
			}
			if u.Scheme == "" || u.Host == "" {
				return fmt.Errorf("server %q: must be an absolute URL with scheme and host", srvName)
			}
		case "stdio":
			return fmt.Errorf("server %q: transport type %q is not yet supported", srvName, srvConfig.Transport.Type)
		default:
			return fmt.Errorf("server %q: invalid transport type %q", srvName, srvConfig.Transport.Type)
		}
		if srvConfig.Transport.Timeout == "" {
			return fmt.Errorf("server %q: empty transport timeout", srvName)
		}
	}
	return nil
}
