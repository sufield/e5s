package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// applyEnvOverrides overrides config values with environment variables if set.
// Returns error for invalid environment variable values to fail fast.
func applyEnvOverrides(cfg *MTLSConfig) error {
	// SPIRE configuration
	if socketPath := os.Getenv("SPIRE_AGENT_SOCKET"); socketPath != "" {
		cfg.SPIRE.SocketPath = socketPath
	}
	if trustDomain := os.Getenv("SPIRE_TRUST_DOMAIN"); trustDomain != "" {
		cfg.SPIRE.TrustDomain = trustDomain
	}
	if timeout := os.Getenv("SPIRE_TIMEOUT"); timeout != "" {
		t, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("invalid SPIRE_TIMEOUT %q: %w", timeout, err)
		}
		cfg.SPIRE.Timeout = t
	}

	// HTTP configuration
	if address := os.Getenv("HTTP_ADDRESS"); address != "" {
		cfg.HTTP.Address = address
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

	// HTTP timeout overrides
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
