package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Load loads config from a YAML file (path), from stdin if path == "-",
// or from environment only if path == "".
// It then applies env overrides, build-specific defaults, and validates.
//
// Usage:
//   - File:     cfg, err := config.Load("config.yaml")
//   - Stdin:    cfg, err := config.Load("-")
//   - Env-only: cfg, err := config.Load("")
//
// Breaking Change: This replaces the previous LoadFromFile and LoadFromEnv functions.
// The new unified Load function handles all cases with a single parameter.
//
// YAML Validation: Unknown keys in the YAML file will cause an error. This prevents
// typos and configuration mistakes from silently being ignored.
func Load(path string) (*MTLSConfig, error) {
	var cfg MTLSConfig

	// Optional YAML source
	if path != "" {
		var r io.ReadCloser
		var err error
		if path == "-" {
			r = os.Stdin
		} else {
			r, err = os.Open(path)
			if err != nil {
				return nil, fmt.Errorf("open %q: %w", path, err)
			}
			defer r.Close()
		}
		dec := yaml.NewDecoder(r)
		dec.KnownFields(true) // Reject unknown YAML keys
		if err := dec.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("parse %q: %w", path, err)
		}
	}

	// Env → defaults → validate
	if err := applyEnvOverrides(&cfg); err != nil {
		return nil, fmt.Errorf("env overrides: %w", err)
	}
	ApplyDefaults(&cfg)
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}
	return &cfg, nil
}
