package server

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ProjectInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ServiceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ServiceRoute struct {
	Cluster string
	Address string
}

type IConfig interface {
	ValidToken(token string) bool
	ClientName(token string) string
	ClientProjects(token string) ([]ProjectInfo, error)
	ClientServices(token, projectID string) ([]ServiceInfo, error)
	ResolveService(token, projectID, serviceID string) (ServiceRoute, error)
	ValidAgentToken(token string) (clusterID string, ok bool)
}

// --- YAML Config ---

type YAMLConfig struct {
	Addr    string         `yaml:"addr"`
	Agents  []AgentConfig  `yaml:"agents"`
	Clients []ClientConfig `yaml:"clients"`
}

type AgentConfig struct {
	Cluster string `yaml:"cluster"`
	Token   string `yaml:"token"`
}

type ProjectConfig struct {
	ID       string          `yaml:"id"`
	Name     string          `yaml:"name"`
	Services []ServiceConfig `yaml:"services"`
}

type ServiceConfig struct {
	ID      string `yaml:"id"`
	Name    string `yaml:"name"`
	Cluster string `yaml:"cluster"`
	Address string `yaml:"address"`
}

type ClientConfig struct {
	Name     string          `yaml:"name"`
	Token    string          `yaml:"token"`
	Projects []ProjectConfig `yaml:"projects"`
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

func (c *YAMLConfig) ClientProjects(token string) ([]ProjectInfo, error) {
	for _, cl := range c.Clients {
		if cl.Token == token {
			infos := make([]ProjectInfo, len(cl.Projects))
			for i, p := range cl.Projects {
				infos[i] = ProjectInfo{ID: p.ID, Name: p.Name}
			}
			return infos, nil
		}
	}
	return nil, fmt.Errorf("invalid token")
}

func (c *YAMLConfig) ClientServices(token, projectID string) ([]ServiceInfo, error) {
	for _, cl := range c.Clients {
		if cl.Token == token {
			for _, p := range cl.Projects {
				if p.ID == projectID {
					infos := make([]ServiceInfo, len(p.Services))
					for i, s := range p.Services {
						infos[i] = ServiceInfo{ID: s.ID, Name: s.Name}
					}
					return infos, nil
				}
			}
			return nil, fmt.Errorf("project not found: %s", projectID)
		}
	}
	return nil, fmt.Errorf("invalid token")
}

func (c *YAMLConfig) ResolveService(token, projectID, serviceID string) (ServiceRoute, error) {
	for _, cl := range c.Clients {
		if cl.Token == token {
			for _, p := range cl.Projects {
				if p.ID == projectID {
					for _, s := range p.Services {
						if s.ID == serviceID {
							return ServiceRoute{Cluster: s.Cluster, Address: s.Address}, nil
						}
					}
					return ServiceRoute{}, fmt.Errorf("service not allowed: %s", serviceID)
				}
			}
			return ServiceRoute{}, fmt.Errorf("project not found: %s", projectID)
		}
	}
	return ServiceRoute{}, fmt.Errorf("invalid token")
}

func (c *YAMLConfig) ValidAgentToken(token string) (string, bool) {
	for _, a := range c.Agents {
		if a.Token == token {
			return a.Cluster, true
		}
	}
	return "", false
}
