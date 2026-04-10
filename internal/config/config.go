// Package config loads the fs-inspect cluster inventory from a YAML file.
//
// The config file is the source of truth for "which FreeSWITCH instances
// does this CLI know about". Everything downstream (reg lookup, channel
// listing, tail) fans out across the nodes defined here.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Node describes a single FreeSWITCH instance reachable over inbound ESL.
type Node struct {
	Name     string `yaml:"name"`
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
}

// Config is the top-level shape of fs-inspect.yaml.
type Config struct {
	Nodes []Node `yaml:"nodes"`
}

// Load reads and parses a YAML config file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if len(cfg.Nodes) == 0 {
		return nil, fmt.Errorf("config %s has no nodes", path)
	}
	for i, n := range cfg.Nodes {
		if n.Name == "" || n.Addr == "" {
			return nil, fmt.Errorf("config %s: node[%d] missing name or addr", path, i)
		}
		if n.Password == "" {
			cfg.Nodes[i].Password = "ClueCon"
		}
	}
	return &cfg, nil
}
