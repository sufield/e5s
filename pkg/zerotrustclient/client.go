package zerotrustclient

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/httpclient"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Client is a zero-config mTLS HTTP client that automatically:
// - Detects the SPIRE agent socket
// - Fetches X.509 SVIDs from the SPIRE Workload API
// - Rotates certificates with zero downtime
// - Verifies server identity using SPIFFE IDs
// - Enforces TLS 1.3+
//
// Usage:
//
//	client, err := zerotrustclient.New(ctx, &zerotrustclient.Config{
//	    ServerID: "spiffe://example.org/server",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	resp, err := client.Get(ctx, "https://localhost:8443/api/hello")
type Client struct {
	internal ports.MTLSClient
}

// Config holds optional configuration for the zero-trust client.
// All fields are optional - sensible defaults are applied.
type Config struct {
	// ServerID is the expected SPIFFE ID of the server.
	// Example: "spiffe://example.org/server"
	// If empty, any server in the same trust domain is accepted.
	ServerID string

	// ServerTrustDomain accepts any server in the specified trust domain.
	// Example: "example.org"
	// Mutually exclusive with ServerID.
	ServerTrustDomain string

	// SocketPath is the path to the SPIRE agent socket.
	// If empty, auto-detects from:
	// - SPIFFE_ENDPOINT_SOCKET env var
	// - SPIRE_AGENT_SOCKET env var
	// - Common paths: /tmp/spire-agent/public/api.sock, /var/run/spire/sockets/agent.sock
	SocketPath string
}

// New creates a new zero-trust mTLS client.
//
// The client automatically handles certificate rotation and server verification.
// Configuration is optional - pass nil or an empty Config for full auto-detection.
//
// If both ServerID and ServerTrustDomain are empty, the client will accept
// any server in the same trust domain as the client's SVID.
func New(ctx context.Context, cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	// Build configuration with auto-detection
	mtlsCfg, err := buildConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build client config: %w", err)
	}

	// Create the underlying mTLS client
	internal, err := httpclient.New(ctx, mtlsCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create mTLS client: %w", err)
	}

	return &Client{internal: internal}, nil
}

// Do executes an HTTP request using identity-based mTLS.
//
// The request is authenticated using the workload's X.509 SVID.
// The server's SPIFFE ID is verified according to the configured policy.
//
// Note: SPIFFE authentication verifies the server's SPIFFE ID, not the DNS hostname.
// Using "localhost" or IP addresses in URLs is fine - the SPIFFE ID is what matters.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return c.internal.Do(ctx, req)
}

// Get is a convenience method that performs a GET request.
//
// It creates the HTTP request and calls Do().
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}
	return c.Do(ctx, req)
}

// Post is a convenience method that performs a POST request.
// The body parameter should be an io.Reader or nil for no body.
func (c *Client) Post(ctx context.Context, url, contentType string, body interface {
	Read([]byte) (int, error)
}) (*http.Response, error) {
	bodyReader := body
	if bodyReader == nil {
		bodyReader = http.NoBody
	}

	req, err := http.NewRequest(http.MethodPost, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return c.Do(ctx, req)
}

// Close releases resources (X509Source, connections, etc.).
// Idempotent and safe to call multiple times.
//
// After Close, subsequent calls to Do/Get/Post will return an error.
func (c *Client) Close() error {
	return c.internal.Close()
}
