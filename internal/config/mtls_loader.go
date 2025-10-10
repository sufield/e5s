package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadFromFile loads configuration from a YAML file with env overrides
func LoadFromFile(path string) (*MTLSConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg MTLSConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Apply environment variable overrides
	if err := applyEnvOverrides(&cfg); err != nil {
		return nil, fmt.Errorf("apply env overrides: %w", err)
	}

	// Set defaults
	applyDefaults(&cfg)

	return &cfg, nil
}

// LoadFromEnv loads configuration from environment variables only
func LoadFromEnv() (*MTLSConfig, error) {
	cfg := &MTLSConfig{}
	if err := applyEnvOverrides(cfg); err != nil {
		return nil, fmt.Errorf("load from env: %w", err)
	}
	applyDefaults(cfg)
	return cfg, nil
}
