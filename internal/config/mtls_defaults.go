package config

import "fmt"

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
