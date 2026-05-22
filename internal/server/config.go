package server

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Addr    string         `yaml:"addr"`
	Clients []ClientConfig `yaml:"clients"`
}

type ServiceConfig struct {
	ID      string `yaml:"id"`
	Address string `yaml:"address"`
}

type ClientConfig struct {
	Name     string          `yaml:"name"`
	Token    string          `yaml:"token"`
	Services []ServiceConfig `yaml:"services"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	return &cfg, nil
}

// ResolveService returns the real address for a service ID under a given token.
// Returns empty string if token is invalid or service not allowed.
func (c *Config) ResolveService(token, serviceID string) string {
	for _, cl := range c.Clients {
		if cl.Token == token {
			for _, s := range cl.Services {
				if s.ID == serviceID {
					return s.Address
				}
			}
			return ""
		}
	}
	return ""
}

// ValidToken checks if a token exists in config.
func (c *Config) ValidToken(token string) bool {
	for _, cl := range c.Clients {
		if cl.Token == token {
			return true
		}
	}
	return false
}

// ClientName returns the name for a given token.
func (c *Config) ClientName(token string) string {
	for _, cl := range c.Clients {
		if cl.Token == token {
			return cl.Name
		}
	}
	return ""
}
