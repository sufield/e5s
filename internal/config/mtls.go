package config

import (
	"fmt"
	"os"
	"time"

	"github.com/pocket/hexagon/spire/internal/ports"
	"gopkg.in/yaml.v3"
)

// MTLSConfig holds configuration for mTLS server and client
type MTLSConfig struct {
	HTTP  HTTPConfig  `yaml:"http"`
	SPIRE SPIREConfig `yaml:"spire"`
}

// HTTPConfig holds HTTP server/client configuration
type HTTPConfig struct {
	Enabled bool          `yaml:"enabled"`
	Port    int           `yaml:"port"`
	Address string        `yaml:"address"`
	Timeout time.Duration `yaml:"timeout"`
	Auth    AuthConfig    `yaml:"authentication"`
}

// SPIREConfig holds SPIRE-specific configuration
type SPIREConfig struct {
	SocketPath  string `yaml:"socket_path"`
	TrustDomain string `yaml:"trust_domain"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	// Policy can be: "any", "trust-domain", "specific-id", "one-of"
	Policy       string   `yaml:"policy"`
	TrustDomain  string   `yaml:"trust_domain"`
	AllowedIDs   []string `yaml:"allowed_ids"`
	AllowedID    string   `yaml:"allowed_id"`
}

// LoadFromFile loads configuration from a YAML file
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
	applyEnvOverrides(&cfg)

	// Set defaults
	applyDefaults(&cfg)

	return &cfg, nil
}

// LoadFromEnv loads configuration from environment variables only
func LoadFromEnv() *MTLSConfig {
	cfg := &MTLSConfig{}
	applyEnvOverrides(cfg)
	applyDefaults(cfg)
	return cfg
}

// applyEnvOverrides overrides config values with environment variables if set
func applyEnvOverrides(cfg *MTLSConfig) {
	// SPIRE configuration
	if socketPath := os.Getenv("SPIRE_AGENT_SOCKET"); socketPath != "" {
		cfg.SPIRE.SocketPath = socketPath
	}
	if trustDomain := os.Getenv("SPIRE_TRUST_DOMAIN"); trustDomain != "" {
		cfg.SPIRE.TrustDomain = trustDomain
	}

	// HTTP configuration
	if address := os.Getenv("HTTP_ADDRESS"); address != "" {
		cfg.HTTP.Address = address
	}
	if enabled := os.Getenv("HTTP_ENABLED"); enabled != "" {
		cfg.HTTP.Enabled = enabled == "true" || enabled == "1"
	}

	// Authentication configuration
	if policy := os.Getenv("AUTH_POLICY"); policy != "" {
		cfg.HTTP.Auth.Policy = policy
	}
	if allowedID := os.Getenv("ALLOWED_CLIENT_ID"); allowedID != "" {
		cfg.HTTP.Auth.AllowedID = allowedID
	}
	if allowedServerID := os.Getenv("EXPECTED_SERVER_ID"); allowedServerID != "" {
		cfg.HTTP.Auth.AllowedID = allowedServerID
	}
	if trustDomain := os.Getenv("AUTH_TRUST_DOMAIN"); trustDomain != "" {
		cfg.HTTP.Auth.TrustDomain = trustDomain
	}
}

// applyDefaults sets default values for unspecified configuration
func applyDefaults(cfg *MTLSConfig) {
	// SPIRE defaults
	if cfg.SPIRE.SocketPath == "" {
		cfg.SPIRE.SocketPath = "unix:///tmp/spire-agent/public/api.sock"
	}
	if cfg.SPIRE.TrustDomain == "" {
		cfg.SPIRE.TrustDomain = "example.org"
	}

	// HTTP defaults
	if cfg.HTTP.Address == "" {
		if cfg.HTTP.Port != 0 {
			cfg.HTTP.Address = fmt.Sprintf(":%d", cfg.HTTP.Port)
		} else {
			cfg.HTTP.Address = ":8443"
		}
	}
	if cfg.HTTP.Timeout == 0 {
		cfg.HTTP.Timeout = 30 * time.Second
	}

	// Authentication defaults
	if cfg.HTTP.Auth.Policy == "" {
		cfg.HTTP.Auth.Policy = "any"
	}
	if cfg.HTTP.Auth.TrustDomain == "" {
		cfg.HTTP.Auth.TrustDomain = cfg.SPIRE.TrustDomain
	}
}

// Validate checks if the configuration is valid
func (c *MTLSConfig) Validate() error {
	// Validate SPIRE config
	if c.SPIRE.SocketPath == "" {
		return fmt.Errorf("spire.socket_path is required")
	}
	if c.SPIRE.TrustDomain == "" {
		return fmt.Errorf("spire.trust_domain is required")
	}

	// Validate authentication policy
	validPolicies := map[string]bool{
		"any":          true,
		"trust-domain": true,
		"specific-id":  true,
		"one-of":       true,
	}
	if !validPolicies[c.HTTP.Auth.Policy] {
		return fmt.Errorf("invalid auth policy %q, must be one of: any, trust-domain, specific-id, one-of", c.HTTP.Auth.Policy)
	}

	// Validate policy-specific requirements
	if c.HTTP.Auth.Policy == "specific-id" && c.HTTP.Auth.AllowedID == "" {
		return fmt.Errorf("auth policy 'specific-id' requires allowed_id to be set")
	}
	if c.HTTP.Auth.Policy == "one-of" && len(c.HTTP.Auth.AllowedIDs) == 0 {
		return fmt.Errorf("auth policy 'one-of' requires allowed_ids to be set")
	}

	return nil
}

// ToServerConfig converts MTLSConfig to ports.MTLSConfig
func (c *MTLSConfig) ToServerConfig() ports.MTLSConfig {
	return ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: c.SPIRE.SocketPath,
		},
		SPIFFE: ports.SPIFFEConfig{
			AllowedPeerID: c.HTTP.Auth.AllowedID,
		},
		HTTP: ports.HTTPConfig{
			Address:           c.HTTP.Address,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      c.HTTP.Timeout,
			IdleTimeout:       120 * time.Second,
		},
	}
}

// ToClientConfig converts MTLSConfig to ports.MTLSConfig
func (c *MTLSConfig) ToClientConfig() ports.MTLSConfig {
	return ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: c.SPIRE.SocketPath,
		},
		SPIFFE: ports.SPIFFEConfig{
			AllowedPeerID: c.HTTP.Auth.AllowedID,
		},
		HTTP: ports.HTTPConfig{
			Timeout: c.HTTP.Timeout,
		},
	}
}
