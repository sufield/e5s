package httpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"

	"github.com/sufield/e5s/internal/ports"
)

// Client implements ports.MTLSClient using the go-spiffe SDK.
// It provides automatic certificate rotation and mTLS authentication.
type Client struct {
	client *http.Client
	source *workloadapi.X509Source
	mu     sync.RWMutex
	closed bool
}

// New creates a new mTLS HTTP client.
//
// The client automatically:
// - Fetches X.509 SVIDs from the SPIRE Workload API
// - Rotates certificates with zero downtime
// - Verifies server identity using SPIFFE ID
// - Enforces TLS 1.3+
//
// Configuration validation:
// - WorkloadAPI.SocketPath must be non-empty
// - Exactly one of SPIFFE.AllowedPeerID or SPIFFE.AllowedTrustDomain must be set
// - HTTP timeouts are optional (uses http.DefaultClient defaults if unset)
//
// The caller must call Close() to release resources.
func New(ctx context.Context, cfg *ports.MTLSConfig) (ports.MTLSClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Validate socket path
	if cfg.WorkloadAPI.SocketPath == "" {
		return nil, fmt.Errorf("WorkloadAPI.SocketPath is required")
	}

	// Validate SPIFFE config (exactly one must be set)
	hasPeerID := cfg.SPIFFE.AllowedPeerID != ""
	hasTrustDomain := cfg.SPIFFE.AllowedTrustDomain != ""

	if !hasPeerID && !hasTrustDomain {
		return nil, fmt.Errorf("either SPIFFE.AllowedPeerID or SPIFFE.AllowedTrustDomain must be set")
	}
	if hasPeerID && hasTrustDomain {
		return nil, fmt.Errorf("cannot set both SPIFFE.AllowedPeerID and SPIFFE.AllowedTrustDomain")
	}

	// Validate SPIFFE ID/trust domain formats BEFORE creating X509Source
	var serverID spiffeid.ID
	var trustDomain spiffeid.TrustDomain
	if hasPeerID {
		var err error
		serverID, err = spiffeid.FromString(cfg.SPIFFE.AllowedPeerID)
		if err != nil {
			return nil, fmt.Errorf("invalid AllowedPeerID: %w", err)
		}
	} else {
		var err error
		trustDomain, err = spiffeid.TrustDomainFromString(cfg.SPIFFE.AllowedTrustDomain)
		if err != nil {
			return nil, fmt.Errorf("invalid AllowedTrustDomain: %w", err)
		}
	}

	// Create X.509 source for automatic SVID rotation
	clientOpts := workloadapi.WithClientOptions(workloadapi.WithAddr(cfg.WorkloadAPI.SocketPath))
	source, err := workloadapi.NewX509Source(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create X.509 source: %w", err)
	}

	// Build TLS config with SPIFFE authorization
	var tlsConfig *tls.Config
	if hasPeerID {
		tlsConfig = tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeID(serverID))
	} else {
		tlsConfig = tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeMemberOf(trustDomain))
	}

	// Enforce TLS 1.3+
	tlsConfig.MinVersion = tls.VersionTLS13

	// Create HTTP client with mTLS transport
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	httpClient := &http.Client{
		Transport: transport,
	}

	// Apply HTTP timeouts if configured
	if cfg.HTTP.IdleTimeout > 0 {
		transport.IdleConnTimeout = cfg.HTTP.IdleTimeout
	}
	if cfg.HTTP.ReadTimeout > 0 || cfg.HTTP.WriteTimeout > 0 {
		// Note: http.Client timeout applies to entire request (read + write)
		// Use the larger of the two as the overall timeout
		timeout := cfg.HTTP.ReadTimeout
		if cfg.HTTP.WriteTimeout > timeout {
			timeout = cfg.HTTP.WriteTimeout
		}
		if timeout > 0 {
			httpClient.Timeout = timeout
		}
	}

	return &Client{
		client: httpClient,
		source: source,
	}, nil
}

// Do executes an HTTP request using identity-based mTLS.
//
// The request is authenticated using the workload's X.509 SVID.
// The server's SPIFFE ID is verified according to the configured authorization policy.
//
// Context cancellation is propagated to the underlying HTTP request.
//
// Security notes:
// - SPIFFE authentication verifies the server's SPIFFE ID (via AuthorizeID or AuthorizeMemberOf)
// - DNS hostname in the URL is NOT used for authentication (only for routing)
// - Using "localhost" or IP addresses is fine - SPIFFE ID is what matters
//
// Example:
//
//	req, _ := http.NewRequest("GET", "https://localhost:8443/api/hello", http.NoBody)
//	resp, err := client.Do(ctx, req)
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	// Add context to request
	req = req.WithContext(ctx)

	// Execute request with mTLS
	return c.client.Do(req)
}

// Close releases resources (X509Source, connections, etc.).
// Idempotent and safe to call multiple times.
//
// After Close, subsequent calls to Do will return an error.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil // Already closed
	}

	c.closed = true

	// Close X.509 source (stops certificate rotation)
	if c.source != nil {
		if err := c.source.Close(); err != nil {
			return fmt.Errorf("failed to close X.509 source: %w", err)
		}
	}

	// Close idle connections
	if c.client != nil && c.client.Transport != nil {
		if transport, ok := c.client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}

	return nil
}
