package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrInvalidConfig is returned when configuration validation fails
	ErrInvalidConfig = errors.New("invalid config")
)

// MTLSConfig holds configuration for mTLS server and client.
type MTLSConfig struct {
	HTTP  HTTPConfig  `yaml:"http"`
	SPIRE SPIREConfig `yaml:"spire"`
}

// HTTPConfig holds HTTP server/client configuration.
type HTTPConfig struct {
	Address           string        `yaml:"address"` // "host:port" format required
	Timeout           time.Duration `yaml:"timeout"` // end-to-end request timeout
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	Auth              AuthConfig    `yaml:"authentication"`
}

// SPIREConfig holds SPIRE-specific configuration.
type SPIREConfig struct {
	SocketPath  string        `yaml:"socket_path"`
	TrustDomain string        `yaml:"trust_domain"`
	Timeout     time.Duration `yaml:"timeout"` // Workload API dial/operation timeout
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	// PeerVerification determines how peer identities are verified.
	// Valid values: "any", "trust-domain", "specific-id", "one-of"
	PeerVerification string   `yaml:"peer_verification"`
	TrustDomain      string   `yaml:"trust_domain"`
	AllowedIDs       []string `yaml:"allowed_ids"`
}

// Validate checks if the configuration is valid.
func (c *MTLSConfig) Validate() error {
	if err := c.HTTP.Validate(); err != nil {
		return fmt.Errorf("%w: http: %w", ErrInvalidConfig, err)
	}
	if err := c.SPIRE.Validate(); err != nil {
		return fmt.Errorf("%w: spire: %w", ErrInvalidConfig, err)
	}
	return nil
}

// Validate checks if HTTP configuration is valid.
func (h *HTTPConfig) Validate() error {
	// Validate address format (host:port required)
	if strings.TrimSpace(h.Address) == "" {
		return fmt.Errorf("address is required")
	}
	host, portStr, err := net.SplitHostPort(h.Address)
	if err != nil {
		return fmt.Errorf("address %q must be host:port format: %w", h.Address, err)
	}
	if portStr == "" {
		return fmt.Errorf("address %q missing port", h.Address)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %q (must be 1-65535)", portStr)
	}
	_ = host // host can be empty (":8443") - allowed syntactically

	// Validate timeouts are non-negative
	if h.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got %v", h.Timeout)
	}
	if h.ReadHeaderTimeout < 0 {
		return fmt.Errorf("read_header_timeout must be non-negative, got %v", h.ReadHeaderTimeout)
	}
	if h.ReadTimeout < 0 {
		return fmt.Errorf("read_timeout must be non-negative, got %v", h.ReadTimeout)
	}
	if h.WriteTimeout < 0 {
		return fmt.Errorf("write_timeout must be non-negative, got %v", h.WriteTimeout)
	}
	if h.IdleTimeout < 0 {
		return fmt.Errorf("idle_timeout must be non-negative, got %v", h.IdleTimeout)
	}

	// Validate authentication config
	return h.Auth.Validate()
}

// Validate checks if authentication configuration is valid.
func (a *AuthConfig) Validate() error {
	// Peer verification mode is required
	if strings.TrimSpace(a.PeerVerification) == "" {
		return fmt.Errorf("peer_verification is required")
	}

	// Validate peer verification mode
	mode := strings.ToLower(a.PeerVerification)
	switch mode {
	case "any", "trust-domain", "specific-id", "one-of":
		// valid modes
	default:
		return fmt.Errorf("invalid peer_verification %q (expected: any, trust-domain, specific-id, one-of)", a.PeerVerification)
	}

	// Validate mode-specific requirements
	switch mode {
	case "specific-id":
		if len(a.AllowedIDs) != 1 {
			return fmt.Errorf("peer_verification=specific-id requires exactly one allowed_ids entry, got %d", len(a.AllowedIDs))
		}
	case "trust-domain":
		if strings.TrimSpace(a.TrustDomain) == "" {
			return fmt.Errorf("peer_verification=trust-domain requires trust_domain to be set")
		}
	case "one-of":
		if len(a.AllowedIDs) < 2 {
			return fmt.Errorf("peer_verification=one-of requires at least 2 allowed_ids entries (use specific-id for single ID), got %d", len(a.AllowedIDs))
		}
	}

	// Validate SPIFFE ID formats
	for i, id := range a.AllowedIDs {
		if !strings.HasPrefix(id, "spiffe://") {
			return fmt.Errorf("allowed_ids[%d] must start with 'spiffe://', got %q", i, id)
		}
	}

	return nil
}

// Validate checks if SPIRE configuration is valid.
func (s *SPIREConfig) Validate() error {
	if strings.TrimSpace(s.SocketPath) == "" {
		return fmt.Errorf("socket_path is required")
	}
	if !strings.HasPrefix(s.SocketPath, "unix://") {
		return fmt.Errorf("socket_path must start with 'unix://', got %q", s.SocketPath)
	}
	if strings.TrimSpace(s.TrustDomain) == "" {
		return fmt.Errorf("trust_domain is required")
	}
	if strings.Contains(s.TrustDomain, "://") {
		return fmt.Errorf("trust_domain must not contain scheme, got %q", s.TrustDomain)
	}
	if strings.Contains(s.TrustDomain, "/") {
		return fmt.Errorf("trust_domain must not contain path, got %q", s.TrustDomain)
	}
	if s.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got %v", s.Timeout)
	}
	return nil
}
