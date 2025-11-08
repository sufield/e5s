package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// CurrentConfigVersion is the current e5s config format version.
	// Version 1 is the initial format.
	CurrentConfigVersion = 1
)

// Load reads and parses an e5s configuration file.
func Load(path string) (FileConfig, error) {
	// Clean the path to prevent directory traversal attacks
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath) // #nosec G304 - Config file path is trusted (from admin/user)
	if err != nil {
		return FileConfig{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return FileConfig{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate version if specified
	// Version 0 (unspecified) is treated as version 1 for backward compatibility
	if cfg.Version != 0 && cfg.Version != CurrentConfigVersion {
		return FileConfig{}, fmt.Errorf("unsupported config version %d (current version: %d)", cfg.Version, CurrentConfigVersion)
	}

	return cfg, nil
}
