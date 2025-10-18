package ports

import (
	"context"
	"net/http"
	"time"
)

// ----- Configuration DTOs (no behavior) -----

// MTLSConfig holds only configuration (no behavior).
type MTLSConfig struct {
	WorkloadAPI WorkloadAPIConfig
	SPIFFE      SPIFFEConfig
	HTTP        HTTPConfig
}

// WorkloadAPIConfig holds Workload API connection configuration.
type WorkloadAPIConfig struct {
	// Example: "unix:///tmp/spire-agent/public/api.sock"
	SocketPath string
}

// SPIFFEConfig holds SPIFFE identity verification configuration (shared for client/server).
type SPIFFEConfig struct {
	// Exactly one of these may be set; precedence is adapter-defined.
	// AllowedPeerID restricts to specific SPIFFE ID (exact match)
	// Example: "spiffe://example.org/client"
	AllowedPeerID string

	// AllowedTrustDomain restricts to specific trust domain (any ID in domain)
	// Example: "example.org"
	AllowedTrustDomain string
}

// HTTPConfig holds HTTP server/client configuration.
type HTTPConfig struct {
	Address           string // e.g., ":8443"
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	// Adapter may ignore unset/zero values and apply its own defaults.
}

// ----- Interfaces only (no helpers/sugar/defaults) -----

// MTLSServer serves HTTPS with SPIFFE/mTLS auth.
// Start MUST block until the server stops (no separate Wait()).
type MTLSServer interface {
	// Handle registers an HTTP handler (same semantics as http.ServeMux).
	// Handlers receive requests with authenticated SPIFFE ID in context.
	// Must be called before Start.
	Handle(pattern string, handler http.Handler) error

	// Start begins serving HTTPS with identity-based mTLS.
	// Blocks until shutdown (graceful or error).
	Start(ctx context.Context) error

	// Shutdown requests a graceful stop.
	Shutdown(ctx context.Context) error

	// Close releases resources (X509Source, connections, etc.).
	// Idempotent.
	Close() error
}

// MTLSClient performs HTTP over SPIFFE/mTLS.
type MTLSClient interface {
	// Do executes an HTTP request using identity-based mTLS.
	Do(ctx context.Context, req *http.Request) (*http.Response, error)

	// Close releases resources (X509Source, connections, etc.).
	// Idempotent.
	Close() error
}
