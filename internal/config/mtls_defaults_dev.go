//go:build dev

package config

import "time"

// Development-only defaults (convenient but explicit).
//
// SECURITY WARNING: These defaults are NOT safe for production:
//   - Binds to all interfaces (":8443")
//   - Uses example trust domain
const (
	DefaultSPIRESocket       = "unix:///tmp/spire-agent/public/api.sock"
	DefaultTrustDomain       = "example.org"
	DefaultHTTPAddress       = ":8443" // binds all interfaces in dev
	DefaultHTTPTimeout       = 30 * time.Second
	DefaultReadHeaderTimeout = 10 * time.Second
	DefaultReadTimeout       = 30 * time.Second
	DefaultWriteTimeout      = 30 * time.Second
	DefaultIdleTimeout       = 120 * time.Second
	DefaultSPIRETimeout      = 30 * time.Second
)

// ApplyDefaults sets safe infrastructure defaults only (dev mode).
//
// Breaking changes from previous version:
//   - No fallback from HTTP.Port to Address (set Address explicitly)
//   - Does NOT copy SPIRE.TrustDomain into HTTP.Auth.TrustDomain
//   - Does NOT default PeerVerification to "any"
//
// Authentication fields are left as-is; validation will fail if required fields are missing.
func ApplyDefaults(cfg *MTLSConfig) {
	// SPIRE defaults
	if cfg.SPIRE.SocketPath == "" {
		cfg.SPIRE.SocketPath = DefaultSPIRESocket
	}
	if cfg.SPIRE.TrustDomain == "" {
		cfg.SPIRE.TrustDomain = DefaultTrustDomain
	}
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
