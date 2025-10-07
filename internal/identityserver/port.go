package identityserver

import (
	"context"
	"net/http"
	"time"
)

// Server is the port interface for an mTLS HTTP server that authenticates clients.
// This interface hides all SPIFFE/SPIRE implementation details from the application.
// Applications depend on this interface, not on the concrete implementation.
type Server interface {
	// Handle registers an HTTP handler for the given pattern.
	// The handler receives requests from authenticated clients.
	// Client identity verification is performed by the server before calling the handler.
	Handle(pattern string, handler http.Handler)

	// Start starts the mTLS HTTP server.
	// The server will authenticate clients using X.509 SVIDs.
	// Returns an error if the server fails to start.
	Start(ctx context.Context) error

	// Shutdown gracefully shuts down the server without interrupting active connections.
	Shutdown(ctx context.Context) error

	// Close immediately closes all connections and releases resources.
	Close() error
}

// Config contains all configuration for creating an identity server.
// This is pure data - it can be loaded from files, environment variables, or any source.
type Config struct {
	// WorkloadAPI configuration for connecting to SPIRE agent
	WorkloadAPI WorkloadAPIConfig

	// SPIFFE configuration for client authentication
	SPIFFE SPIFFEConfig

	// HTTP server configuration
	HTTP HTTPConfig
}

// WorkloadAPIConfig configures the connection to the SPIRE Workload API.
type WorkloadAPIConfig struct {
	// SocketPath is the path to the SPIRE agent's Unix domain socket.
	// Example: "unix:///tmp/spire-agent/public/api.sock"
	SocketPath string
}

// SPIFFEConfig configures client authentication using SPIFFE IDs.
type SPIFFEConfig struct {
	// AllowedClientID is the SPIFFE ID that clients must present.
	// If empty, any client from the trust domain is allowed.
	// Example: "spiffe://example.org/client"
	AllowedClientID string

	// AllowedTrustDomain restricts clients to a specific trust domain.
	// If empty, uses the server's trust domain.
	// Example: "example.org"
	AllowedTrustDomain string
}

// HTTPConfig configures the HTTP server behavior.
type HTTPConfig struct {
	// Address is the address to listen on.
	// Example: ":8443"
	Address string

	// ReadHeaderTimeout is the maximum time to read request headers.
	// Helps prevent slowloris attacks.
	ReadHeaderTimeout time.Duration

	// ReadTimeout is the maximum time to read the entire request.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum time to write the response.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum time to wait for the next request.
	IdleTimeout time.Duration
}

// DefaultConfig returns a configuration with reasonable defaults.
func DefaultConfig() Config {
	return Config{
		WorkloadAPI: WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		SPIFFE: SPIFFEConfig{
			AllowedClientID:    "", // Allow any client from trust domain
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
