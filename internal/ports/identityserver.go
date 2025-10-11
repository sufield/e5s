package ports

import (
	"context"
	"io"
	"net/http"
	"time"
)

// MTLSConfig holds only configuration (no behavior).
type MTLSConfig struct {
	WorkloadAPI WorkloadAPIConfig
	SPIFFE      SPIFFEConfig
	HTTP        HTTPConfig
}

// WorkloadAPIConfig holds Workload API connection configuration.
type WorkloadAPIConfig struct {
	SocketPath string // e.g., "unix:///tmp/spire-agent/public/api.sock"
}

// SPIFFEConfig holds SPIFFE identity verification configuration (shared for client/server).
type SPIFFEConfig struct {
	// AllowedPeerID restricts to specific SPIFFE ID (exact match)
	// Example: "spiffe://example.org/client"
	AllowedPeerID string

	// AllowedTrustDomain restricts to specific trust domain (any ID in domain)
	// Example: "example.org"
	// If both are empty, any authenticated peer from the same trust domain is allowed
	AllowedTrustDomain string
}

// HTTPConfig holds HTTP server/client configuration.
type HTTPConfig struct {
	Address           string        // e.g., ":8443"
	ReadHeaderTimeout time.Duration // e.g., 10 * time.Second (prevents Slowloris)
	ReadTimeout       time.Duration // e.g., 30 * time.Second
	WriteTimeout      time.Duration // e.g., 30 * time.Second
	IdleTimeout       time.Duration // e.g., 120 * time.Second
	Timeout           time.Duration // Client-specific timeout, e.g., 30 * time.Second
}

// MTLSServer is the stable interface for an mTLS HTTP server.
// It provides identity-based authentication using SPIFFE/SPIRE.
type MTLSServer interface {
	// Handle registers an HTTP handler (same semantics as http.ServeMux).
	// Handlers receive requests with authenticated SPIFFE ID in context.
	Handle(pattern string, handler http.Handler)
	// Start begins serving HTTPS with identity-based mTLS.
	Start(ctx context.Context) error
	// Shutdown gracefully stops the server, waiting for active connections.
	Shutdown(ctx context.Context) error
	// Close releases resources (X509Source, connections, etc.).
	Close() error
	// GetMux returns the underlying ServeMux for advanced use.
	GetMux() *http.ServeMux
}

// MTLSClient is the stable interface for an mTLS HTTP client.
// It provides identity-based authentication and server verification using SPIFFE/SPIRE.
type MTLSClient interface {
	// Do executes an HTTP request using identity-based mTLS.
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
	// Get is a convenience for simple GET requests.
	Get(ctx context.Context, url string) (*http.Response, error)
	// Post is a convenience for POST requests.
	Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error)
	// Close releases resources (X509Source, connections, etc.).
	Close() error
}

// DefaultMTLSConfig returns a configuration with reasonable defaults.
func DefaultMTLSConfig() MTLSConfig {
	return MTLSConfig{
		WorkloadAPI: WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		SPIFFE: SPIFFEConfig{
			AllowedPeerID:      "", // Allow any authenticated peer
			AllowedTrustDomain: "",
		},
		HTTP: HTTPConfig{
			Address:           ":8443",
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
}
