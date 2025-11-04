package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
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

	return cfg, nil
}
