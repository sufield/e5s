//go:build dev

package config

import "time"

// Development-only defaults (convenient, permissive).
//
// SECURITY WARNING: These defaults are NOT safe for production:
//   - Binds to all interfaces (":8443")
//   - Allows any authenticated peer by default
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

// ApplyDefaults sets default values for unspecified configuration (dev mode).
func ApplyDefaults(cfg *MTLSConfig) {
	// SPIRE defaults
	if cfg.SPIRE.SocketPath == "" {
		cfg.SPIRE.SocketPath = DefaultSPIRESocket
	}
	if cfg.SPIRE.TrustDomain == "" {
		cfg.SPIRE.TrustDomain = DefaultTrustDomain
	}
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

	// Authentication defaults (dev convenience: allow any authenticated peer)
	if cfg.HTTP.Auth.PeerVerification == "" {
		cfg.HTTP.Auth.PeerVerification = "any"
	}
	if cfg.HTTP.Auth.TrustDomain == "" {
		cfg.HTTP.Auth.TrustDomain = cfg.SPIRE.TrustDomain
	}
}
