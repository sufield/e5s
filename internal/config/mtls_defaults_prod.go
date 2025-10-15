//go:build !dev

package config

import "time"

// Production defaults (safe, conservative).
//
// Security properties:
//   - Binds to loopback only ("127.0.0.1:8443") by default
//   - No SPIRE defaults - must be explicitly configured
//   - No auth defaults - must be explicitly configured
const (
	DefaultHTTPAddress       = "127.0.0.1:8443" // force loopback unless explicitly changed
	DefaultHTTPTimeout       = 30 * time.Second
	DefaultReadHeaderTimeout = 10 * time.Second
	DefaultReadTimeout       = 30 * time.Second
	DefaultWriteTimeout      = 30 * time.Second
	DefaultIdleTimeout       = 120 * time.Second
	DefaultSPIRETimeout      = 30 * time.Second
)

// ApplyDefaults sets safe infrastructure defaults only (production mode).
//
// Breaking changes from previous version:
//   - No fallback from HTTP.Port to Address (set Address explicitly)
//   - Does NOT copy SPIRE.TrustDomain into HTTP.Auth.TrustDomain
//   - Does NOT default PeerVerification to "any"
//
// In production:
//   - SPIRE socket path and trust domain must be explicitly configured
//   - Authentication fields must be explicitly configured
//   - Validation will fail if required fields are missing
func ApplyDefaults(cfg *MTLSConfig) {
	// SPIRE: no socket path or trust domain defaults in prod (must be explicit)
	if cfg.SPIRE.Timeout == 0 {
		cfg.SPIRE.Timeout = DefaultSPIRETimeout
	}

	// HTTP infrastructure defaults (timeouts, address)
	if cfg.HTTP.Address == "" {
		cfg.HTTP.Address = DefaultHTTPAddress
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

	// Auth: no implicit defaults (force explicit config/validation elsewhere).
	// cfg.HTTP.Auth.PeerVerification stays as provided.
	// cfg.HTTP.Auth.TrustDomain stays as provided.
	// cfg.HTTP.Auth.AllowedIDs stays as provided.
}
