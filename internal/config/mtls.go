package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pocket/hexagon/spire/internal/ports"
	"gopkg.in/yaml.v3"
)

// Default configuration constants
const (
	// SPIRE defaults
	DefaultSPIRESocket = "unix:///tmp/spire-agent/public/api.sock"
	DefaultTrustDomain = "example.org"

	// HTTP defaults
	DefaultHTTPAddress       = ":8443"
	DefaultHTTPPort          = 8443
	DefaultHTTPTimeout       = 30 * time.Second
	DefaultReadHeaderTimeout = 10 * time.Second
	DefaultReadTimeout       = 30 * time.Second
	DefaultWriteTimeout      = 30 * time.Second
	DefaultIdleTimeout       = 120 * time.Second

	// Auth defaults
	DefaultAuthPeerVerification = "any"

	// Port validation
	MinPort = 1
	MaxPort = 65535
)

// MTLSConfig holds configuration for mTLS server and client
type MTLSConfig struct {
	HTTP  HTTPConfig  `yaml:"http"`
	SPIRE SPIREConfig `yaml:"spire"`
}

// HTTPConfig holds HTTP server/client configuration
type HTTPConfig struct {
	Enabled           bool          `yaml:"enabled"`
	Port              int           `yaml:"port"`
	Address           string        `yaml:"address"`
	Timeout           time.Duration `yaml:"timeout"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	Auth              AuthConfig    `yaml:"authentication"`
}

// SPIREConfig holds SPIRE-specific configuration
type SPIREConfig struct {
	SocketPath  string `yaml:"socket_path"`
	TrustDomain string `yaml:"trust_domain"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	// PeerVerification determines how peer identities are verified
	// Valid values: "any", "trust-domain", "specific-id", "one-of"
	PeerVerification string   `yaml:"peer_verification"`
	TrustDomain      string   `yaml:"trust_domain"`
	AllowedIDs       []string `yaml:"allowed_ids"`
	AllowedID        string   `yaml:"allowed_id"`
}

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

// applyEnvOverrides overrides config values with environment variables if set
// Returns error for invalid environment variable values to fail fast
func applyEnvOverrides(cfg *MTLSConfig) error {
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
	if port := os.Getenv("HTTP_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("invalid HTTP_PORT %q: %w", port, err)
		}
		cfg.HTTP.Port = p
	}
	if enabled := os.Getenv("HTTP_ENABLED"); enabled != "" {
		e, err := parseBool(enabled)
		if err != nil {
			return fmt.Errorf("invalid HTTP_ENABLED %q: %w", enabled, err)
		}
		cfg.HTTP.Enabled = e
	}
	if timeout := os.Getenv("HTTP_TIMEOUT"); timeout != "" {
		t, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("invalid HTTP_TIMEOUT %q: %w", timeout, err)
		}
		cfg.HTTP.Timeout = t
	}

	// Timeout overrides
	if timeout := os.Getenv("HTTP_READ_HEADER_TIMEOUT"); timeout != "" {
		t, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("invalid HTTP_READ_HEADER_TIMEOUT %q: %w", timeout, err)
		}
		cfg.HTTP.ReadHeaderTimeout = t
	}
	if timeout := os.Getenv("HTTP_READ_TIMEOUT"); timeout != "" {
		t, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("invalid HTTP_READ_TIMEOUT %q: %w", timeout, err)
		}
		cfg.HTTP.ReadTimeout = t
	}
	if timeout := os.Getenv("HTTP_WRITE_TIMEOUT"); timeout != "" {
		t, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("invalid HTTP_WRITE_TIMEOUT %q: %w", timeout, err)
		}
		cfg.HTTP.WriteTimeout = t
	}
	if timeout := os.Getenv("HTTP_IDLE_TIMEOUT"); timeout != "" {
		t, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("invalid HTTP_IDLE_TIMEOUT %q: %w", timeout, err)
		}
		cfg.HTTP.IdleTimeout = t
	}

	// Authentication configuration
	if peerVerification := os.Getenv("AUTH_PEER_VERIFICATION"); peerVerification != "" {
		cfg.HTTP.Auth.PeerVerification = peerVerification
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
	// Support comma-separated list for AllowedIDs
	if allowedIDs := os.Getenv("ALLOWED_IDS"); allowedIDs != "" {
		cfg.HTTP.Auth.AllowedIDs = strings.Split(allowedIDs, ",")
		// Trim whitespace from each ID
		for i := range cfg.HTTP.Auth.AllowedIDs {
			cfg.HTTP.Auth.AllowedIDs[i] = strings.TrimSpace(cfg.HTTP.Auth.AllowedIDs[i])
		}
	}

	return nil
}

// parseBool parses boolean environment variables
// Accepts: "true", "1", "yes", "on" for true; "false", "0", "no", "off" for false
func parseBool(value string) (bool, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", value)
	}
}

// applyDefaults sets default values for unspecified configuration
func applyDefaults(cfg *MTLSConfig) {
	// SPIRE defaults
	if cfg.SPIRE.SocketPath == "" {
		cfg.SPIRE.SocketPath = DefaultSPIRESocket
	}
	if cfg.SPIRE.TrustDomain == "" {
		cfg.SPIRE.TrustDomain = DefaultTrustDomain
	}

	// HTTP defaults
	if cfg.HTTP.Address == "" {
		if cfg.HTTP.Port != 0 {
			cfg.HTTP.Address = fmt.Sprintf(":%d", cfg.HTTP.Port)
		} else {
			cfg.HTTP.Address = DefaultHTTPAddress
		}
	}
	if cfg.HTTP.Timeout == 0 {
		cfg.HTTP.Timeout = DefaultHTTPTimeout
	}
	if cfg.HTTP.ReadHeaderTimeout == 0 {
		cfg.HTTP.ReadHeaderTimeout = DefaultReadHeaderTimeout
	}
	if cfg.HTTP.ReadTimeout == 0 {
		cfg.HTTP.ReadTimeout = DefaultReadTimeout
	}
	if cfg.HTTP.WriteTimeout == 0 {
		cfg.HTTP.WriteTimeout = DefaultWriteTimeout
	}
	if cfg.HTTP.IdleTimeout == 0 {
		cfg.HTTP.IdleTimeout = DefaultIdleTimeout
	}

	// Authentication defaults
	if cfg.HTTP.Auth.PeerVerification == "" {
		cfg.HTTP.Auth.PeerVerification = DefaultAuthPeerVerification
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
	// Validate socket path format
	if !strings.HasPrefix(c.SPIRE.SocketPath, "unix://") {
		return fmt.Errorf("spire.socket_path must start with 'unix://', got %q", c.SPIRE.SocketPath)
	}
	if c.SPIRE.TrustDomain == "" {
		return fmt.Errorf("spire.trust_domain is required")
	}
	// Basic trust domain validation (DNS-like)
	if strings.Contains(c.SPIRE.TrustDomain, "://") {
		return fmt.Errorf("spire.trust_domain must not contain scheme, got %q", c.SPIRE.TrustDomain)
	}
	if strings.Contains(c.SPIRE.TrustDomain, "/") {
		return fmt.Errorf("spire.trust_domain must not contain path, got %q", c.SPIRE.TrustDomain)
	}

	// Validate HTTP config
	if c.HTTP.Enabled && c.HTTP.Address == "" {
		return fmt.Errorf("http.address is required when http.enabled is true")
	}
	if c.HTTP.Port < 0 || c.HTTP.Port > MaxPort {
		return fmt.Errorf("http.port must be between %d and %d, got %d", MinPort, MaxPort, c.HTTP.Port)
	}
	if c.HTTP.Timeout < 0 {
		return fmt.Errorf("http.timeout must be positive, got %v", c.HTTP.Timeout)
	}
	if c.HTTP.ReadHeaderTimeout < 0 {
		return fmt.Errorf("http.read_header_timeout must be positive, got %v", c.HTTP.ReadHeaderTimeout)
	}
	if c.HTTP.ReadTimeout < 0 {
		return fmt.Errorf("http.read_timeout must be positive, got %v", c.HTTP.ReadTimeout)
	}
	if c.HTTP.WriteTimeout < 0 {
		return fmt.Errorf("http.write_timeout must be positive, got %v", c.HTTP.WriteTimeout)
	}
	if c.HTTP.IdleTimeout < 0 {
		return fmt.Errorf("http.idle_timeout must be positive, got %v", c.HTTP.IdleTimeout)
	}

	// Validate peer verification mode
	validModes := map[string]bool{
		"any":          true,
		"trust-domain": true,
		"specific-id":  true,
		"one-of":       true,
	}
	if !validModes[c.HTTP.Auth.PeerVerification] {
		return fmt.Errorf("invalid peer_verification %q, must be one of: any, trust-domain, specific-id, one-of", c.HTTP.Auth.PeerVerification)
	}

	// Validate peer verification mode-specific requirements
	if c.HTTP.Auth.PeerVerification == "specific-id" && c.HTTP.Auth.AllowedID == "" {
		return fmt.Errorf("peer_verification 'specific-id' requires allowed_id to be set")
	}
	if c.HTTP.Auth.PeerVerification == "one-of" && len(c.HTTP.Auth.AllowedIDs) == 0 {
		return fmt.Errorf("peer_verification 'one-of' requires allowed_ids to be set")
	}
	if c.HTTP.Auth.PeerVerification == "trust-domain" && c.HTTP.Auth.TrustDomain == "" {
		return fmt.Errorf("peer_verification 'trust-domain' requires trust_domain to be set")
	}

	// Validate SPIFFE IDs format (basic check)
	if c.HTTP.Auth.AllowedID != "" && !strings.HasPrefix(c.HTTP.Auth.AllowedID, "spiffe://") {
		return fmt.Errorf("allowed_id must be a valid SPIFFE ID (start with 'spiffe://'), got %q", c.HTTP.Auth.AllowedID)
	}
	for i, id := range c.HTTP.Auth.AllowedIDs {
		if !strings.HasPrefix(id, "spiffe://") {
			return fmt.Errorf("allowed_ids[%d] must be a valid SPIFFE ID (start with 'spiffe://'), got %q", i, id)
		}
	}

	return nil
}

// ToServerConfig converts MTLSConfig to ports.MTLSConfig for server use
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
			ReadHeaderTimeout: c.HTTP.ReadHeaderTimeout,
			WriteTimeout:      c.HTTP.WriteTimeout,
			IdleTimeout:       c.HTTP.IdleTimeout,
		},
	}
}

// ToClientConfig converts MTLSConfig to ports.MTLSConfig for client use
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
