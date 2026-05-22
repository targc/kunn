package server

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ServiceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type IConfig interface {
	ValidToken(token string) bool
	ClientName(token string) string
	ClientServices(token string) ([]ServiceInfo, error)
	ResolveService(token, serviceID string) (string, error)
}

// --- YAML Config ---

type YAMLConfig struct {
	Addr    string         `yaml:"addr"`
	Clients []ClientConfig `yaml:"clients"`
}

type ServiceConfig struct {
	ID      string `yaml:"id"`
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
}

type ClientConfig struct {
	Name     string          `yaml:"name"`
	Token    string          `yaml:"token"`
	Services []ServiceConfig `yaml:"services"`
}

func LoadYAMLConfig(path string) (*YAMLConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	var cfg YAMLConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	return &cfg, nil
}

func (c *YAMLConfig) ValidToken(token string) bool {
	for _, cl := range c.Clients {
		if cl.Token == token {
			return true
		}
	}
	return false
}

func (c *YAMLConfig) ClientName(token string) string {
	for _, cl := range c.Clients {
		if cl.Token == token {
			return cl.Name
		}
	}
	return ""
}

func (c *YAMLConfig) ClientServices(token string) ([]ServiceInfo, error) {
	for _, cl := range c.Clients {
		if cl.Token == token {
			infos := make([]ServiceInfo, len(cl.Services))
			for i, s := range cl.Services {
				infos[i] = ServiceInfo{ID: s.ID, Name: s.Name}
			}
			return infos, nil
		}
	}
	return nil, fmt.Errorf("invalid token")
}

func (c *YAMLConfig) ResolveService(token, serviceID string) (string, error) {
	for _, cl := range c.Clients {
		if cl.Token == token {
			for _, s := range cl.Services {
				if s.ID == serviceID {
					return s.Address, nil
				}
			}
			return "", fmt.Errorf("service not allowed: %s", serviceID)
		}
	}
	return "", fmt.Errorf("invalid token")
}
