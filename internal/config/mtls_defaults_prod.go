//go:build !dev

package config

import "time"

// Production defaults (safe, conservative).
//
// Security properties:
//   - Binds to loopback only ("127.0.0.1:8443") by default
//   - No "any" auth default - must be explicitly configured
//   - Requires explicit SPIRE socket and trust domain configuration
const (
	DefaultHTTPAddress       = "127.0.0.1:8443" // force loopback unless explicitly changed
	DefaultHTTPTimeout       = 30 * time.Second
	DefaultReadHeaderTimeout = 10 * time.Second
	DefaultReadTimeout       = 30 * time.Second
	DefaultWriteTimeout      = 30 * time.Second
	DefaultIdleTimeout       = 120 * time.Second
	DefaultSPIRETimeout      = 30 * time.Second
)

// ApplyDefaults sets default values for unspecified configuration (production mode).
//
// Note: In production, SPIRE socket path and trust domain are NOT defaulted.
// These must be explicitly configured to prevent misconfiguration.
func ApplyDefaults(cfg *MTLSConfig) {
	// SPIRE: no socket path or trust domain defaults in prod (must be explicit)
	if cfg.SPIRE.Timeout <= 0 {
		cfg.SPIRE.Timeout = DefaultSPIRETimeout
	}

	// HTTP defaults
	if cfg.HTTP.Address == "" {
		cfg.HTTP.Address = DefaultHTTPAddress
	}
	if cfg.HTTP.Timeout <= 0 {
		cfg.HTTP.Timeout = DefaultHTTPTimeout
	}
	if cfg.HTTP.ReadHeaderTimeout <= 0 {
		cfg.HTTP.ReadHeaderTimeout = DefaultReadHeaderTimeout
	}
	if cfg.HTTP.ReadTimeout <= 0 {
		cfg.HTTP.ReadTimeout = DefaultReadTimeout
	}
	if cfg.HTTP.WriteTimeout <= 0 {
		cfg.HTTP.WriteTimeout = DefaultWriteTimeout
	}
	if cfg.HTTP.IdleTimeout <= 0 {
		cfg.HTTP.IdleTimeout = DefaultIdleTimeout
	}

	// Authentication: inherit trust domain from SPIRE if not specified
	if cfg.HTTP.Auth.TrustDomain == "" {
		cfg.HTTP.Auth.TrustDomain = cfg.SPIRE.TrustDomain
	}

	// Production: NO default peer verification - must be explicit
}
