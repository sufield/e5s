package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadFromFile loads configuration from a YAML file with env overrides and validation.
//
// Process:
// 1. Parse YAML file
// 2. Apply environment variable overrides
// 3. Apply build-specific defaults (dev vs prod)
// 4. Validate configuration
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

	// Apply build-specific defaults (dev vs prod)
	ApplyDefaults(&cfg)

	// Validate final configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// LoadFromEnv loads configuration from environment variables only with validation.
//
// Process:
// 1. Apply environment variable overrides
// 2. Apply build-specific defaults (dev vs prod)
// 3. Validate configuration
func LoadFromEnv() (*MTLSConfig, error) {
	cfg := &MTLSConfig{}
	if err := applyEnvOverrides(cfg); err != nil {
		return nil, fmt.Errorf("load from env: %w", err)
	}

	// Apply build-specific defaults (dev vs prod)
	ApplyDefaults(cfg)

	// Validate final configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}
