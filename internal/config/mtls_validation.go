package config

import (
	"fmt"
	"strings"
)

// Validate checks if the configuration is valid
func (c *MTLSConfig) Validate() error {
	if err := c.validateSPIREConfig(); err != nil {
		return err
	}
	if err := c.validateHTTPConfig(); err != nil {
		return err
	}
	if err := c.validateAuthConfig(); err != nil {
		return err
	}
	return nil
}

// validateSPIREConfig validates SPIRE socket path and trust domain
func (c *MTLSConfig) validateSPIREConfig() error {
	if c.SPIRE.SocketPath == "" {
		return fmt.Errorf("spire.socket_path is required")
	}
	if !strings.HasPrefix(c.SPIRE.SocketPath, "unix://") {
		return fmt.Errorf("spire.socket_path must start with 'unix://', got %q", c.SPIRE.SocketPath)
	}
	if c.SPIRE.TrustDomain == "" {
		return fmt.Errorf("spire.trust_domain is required")
	}
	if strings.Contains(c.SPIRE.TrustDomain, "://") {
		return fmt.Errorf("spire.trust_domain must not contain scheme, got %q", c.SPIRE.TrustDomain)
	}
	if strings.Contains(c.SPIRE.TrustDomain, "/") {
		return fmt.Errorf("spire.trust_domain must not contain path, got %q", c.SPIRE.TrustDomain)
	}
	return nil
}

// validateHTTPConfig validates HTTP server configuration
func (c *MTLSConfig) validateHTTPConfig() error {
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
	return nil
}

// validateAuthConfig validates authentication and peer verification configuration
func (c *MTLSConfig) validateAuthConfig() error {
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

	// Validate mode-specific requirements
	if err := c.validatePeerVerificationRequirements(); err != nil {
		return err
	}

	// Validate SPIFFE ID formats
	return c.validateSPIFFEIDs()
}

// validatePeerVerificationRequirements validates mode-specific requirements
func (c *MTLSConfig) validatePeerVerificationRequirements() error {
	switch c.HTTP.Auth.PeerVerification {
	case "specific-id":
		if c.HTTP.Auth.AllowedID == "" {
			return fmt.Errorf("peer_verification 'specific-id' requires allowed_id to be set")
		}
	case "one-of":
		if len(c.HTTP.Auth.AllowedIDs) == 0 {
			return fmt.Errorf("peer_verification 'one-of' requires allowed_ids to be set")
		}
	case "trust-domain":
		if c.HTTP.Auth.TrustDomain == "" {
			return fmt.Errorf("peer_verification 'trust-domain' requires trust_domain to be set")
		}
	}
	return nil
}

// validateSPIFFEIDs validates SPIFFE ID format for allowed IDs
func (c *MTLSConfig) validateSPIFFEIDs() error {
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
