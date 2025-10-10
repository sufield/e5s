package config

import "time"

// Default configuration constants
//
// SECURITY NOTE: These defaults are for development/testing only.
// In production environments:
//   - Override DefaultSPIRESocket with your actual SPIRE agent socket path
//   - Override DefaultTrustDomain with your organization's registered trust domain
//   - Override DefaultHTTPAddress to bind to specific interface (e.g., "127.0.0.1:8443")
//   - Always use environment variables or config files to override these values
//   - Never expose :8443 on all interfaces (0.0.0.0) in production
const (
	// SPIRE defaults (DEV ONLY)
	DefaultSPIRESocket = "unix:///tmp/spire-agent/public/api.sock" // Dev: local SPIRE socket
	DefaultTrustDomain = "example.org"                             // Dev: example trust domain - CHANGE IN PROD

	// HTTP defaults (DEV ONLY - bind to specific IP in production)
	DefaultHTTPAddress       = ":8443"            // Dev: binds to all interfaces - USE "127.0.0.1:8443" in prod
	DefaultHTTPPort          = 8443               // Default mTLS port
	DefaultHTTPTimeout       = 30 * time.Second   // Default request timeout
	DefaultReadHeaderTimeout = 10 * time.Second   // Mitigates Slowloris attacks
	DefaultReadTimeout       = 30 * time.Second   // Full request read timeout
	DefaultWriteTimeout      = 30 * time.Second   // Response write timeout
	DefaultIdleTimeout       = 120 * time.Second  // Keep-alive timeout

	// Auth defaults
	DefaultAuthPeerVerification = "any" // Dev: allows any authenticated peer - USE "trust-domain" in prod

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
