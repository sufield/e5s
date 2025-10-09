package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// ClientConfig contains configuration for creating an HTTP client with mTLS.
type ClientConfig struct {
	// SocketPath is the SPIRE agent socket path (e.g., "unix:///tmp/spire-agent/public/api.sock")
	SocketPath string

	// ServerAuthorizer verifies server identity (use tlsconfig.AuthorizeAny(), AuthorizeID(), etc.)
	ServerAuthorizer tlsconfig.Authorizer

	// Timeout for HTTP requests (optional - defaults to 30s)
	Timeout time.Duration

	// Transport settings (optional - defaults provided)
	Transport TransportConfig
}

// TransportConfig contains HTTP transport configuration.
type TransportConfig struct {
	// MaxIdleConns controls the maximum number of idle connections across all hosts (default: 100)
	MaxIdleConns int

	// MaxIdleConnsPerHost controls the maximum idle connections per host (default: 10)
	MaxIdleConnsPerHost int

	// IdleConnTimeout is the maximum time an idle connection is kept (default: 90s)
	IdleConnTimeout time.Duration
}

// SPIFFEHTTPClient is an HTTP client that uses X.509 SVIDs for mTLS authentication.
type SPIFFEHTTPClient struct {
	client     *http.Client
	x509Source *workloadapi.X509Source
}

// NewSPIFFEHTTPClient creates an mTLS HTTP client.
func NewSPIFFEHTTPClient(ctx context.Context, cfg ClientConfig) (*SPIFFEHTTPClient, error) {
	// Validate required fields
	if cfg.SocketPath == "" {
		return nil, fmt.Errorf("socket path is required")
	}
	if cfg.ServerAuthorizer == nil {
		return nil, fmt.Errorf("server authorizer is required")
	}

	// Apply defaults
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Transport.MaxIdleConns == 0 {
		cfg.Transport.MaxIdleConns = 100
	}
	if cfg.Transport.MaxIdleConnsPerHost == 0 {
		cfg.Transport.MaxIdleConnsPerHost = 10
	}
	if cfg.Transport.IdleConnTimeout == 0 {
		cfg.Transport.IdleConnTimeout = 90 * time.Second
	}

	// Create X.509 source from SPIRE Workload API
	// This handles automatic SVID fetching and rotation
	x509Source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(
			workloadapi.WithAddr(cfg.SocketPath),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create X509Source: %w", err)
	}

	// Create mTLS client configuration
	// - Client presents its SVID to server
	// - Server must present valid SVID matching authorizer
	tlsConfig := tlsconfig.MTLSClientConfig(
		x509Source,           // SVID source (client certificate)
		x509Source,           // Bundle source (trusted CAs)
		cfg.ServerAuthorizer, // Server identity verification
	)

	// Create HTTP transport with mTLS
	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        cfg.Transport.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.Transport.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.Transport.IdleConnTimeout,
	}

	// Create HTTP client with configured timeout
	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}

	return &SPIFFEHTTPClient{
		client:     client,
		x509Source: x509Source,
	}, nil
}

// Get performs an HTTP GET request.
func (c *SPIFFEHTTPClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}
	return c.client.Do(req)
}

// Post performs an HTTP POST request.
func (c *SPIFFEHTTPClient) Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	return c.client.Do(req)
}

// Put performs an HTTP PUT request.
func (c *SPIFFEHTTPClient) Put(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	return c.client.Do(req)
}

// Delete performs an HTTP DELETE request.
func (c *SPIFFEHTTPClient) Delete(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create DELETE request: %w", err)
	}
	return c.client.Do(req)
}

// Patch performs an HTTP PATCH request.
func (c *SPIFFEHTTPClient) Patch(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create PATCH request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	return c.client.Do(req)
}

// Do performs an HTTP request.
func (c *SPIFFEHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

// Close releases all resources used by the client.
func (c *SPIFFEHTTPClient) Close() error {
	// Close idle connections
	if c.client != nil {
		c.client.CloseIdleConnections()
	}

	// Close X509Source (stops SVID fetching and rotation)
	if c.x509Source != nil {
		if err := c.x509Source.Close(); err != nil {
			return fmt.Errorf("failed to close X509Source: %w", err)
		}
	}

	return nil
}

// SetTimeout changes the client timeout.
func (c *SPIFFEHTTPClient) SetTimeout(timeout time.Duration) {
	c.client.Timeout = timeout
}

// GetHTTPClient returns the underlying *http.Client for advanced usage.
func (c *SPIFFEHTTPClient) GetHTTPClient() *http.Client {
	return c.client
}
