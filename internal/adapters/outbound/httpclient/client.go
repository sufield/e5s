package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// X509SourceProvider defines how to obtain an X509Source
// Implementations can create from SPIRE Workload API (production) or in-memory (testing)
type X509SourceProvider interface {
	// GetX509Source returns an X509Source for mTLS configuration
	GetX509Source(ctx context.Context) (x509svid.Source, x509bundle.Source, io.Closer, error)
}

// WorkloadAPISourceProvider creates X509Source from SPIRE Workload API (production)
type WorkloadAPISourceProvider struct {
	SocketPath string
}

// GetX509Source implements X509SourceProvider for production SPIRE Workload API
func (p *WorkloadAPISourceProvider) GetX509Source(ctx context.Context) (x509svid.Source, x509bundle.Source, io.Closer, error) {
	x509Source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(
			workloadapi.WithAddr(p.SocketPath),
		),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create X509Source: %w", err)
	}
	// workloadapi.X509Source implements both x509svid.Source and x509bundle.Source
	return x509Source, x509Source, x509Source, nil
}

// InMemorySourceProvider wraps in-memory X509Source (testing/development)
type InMemorySourceProvider struct {
	SVIDSource   x509svid.Source
	BundleSource x509bundle.Source
}

// GetX509Source implements X509SourceProvider for in-memory mode
func (p *InMemorySourceProvider) GetX509Source(ctx context.Context) (x509svid.Source, x509bundle.Source, io.Closer, error) {
	// In-memory sources don't need cleanup
	return p.SVIDSource, p.BundleSource, io.NopCloser(nil), nil
}

// ClientConfig contains configuration for creating an HTTP client with mTLS.
type ClientConfig struct {
	// X509SourceProvider provides the X509Source (production or in-memory)
	X509SourceProvider X509SourceProvider

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
	client *http.Client
	closer io.Closer
}

// NewSPIFFEHTTPClient creates an mTLS HTTP client.
func NewSPIFFEHTTPClient(ctx context.Context, cfg ClientConfig) (*SPIFFEHTTPClient, error) {
	// Validate required fields
	if cfg.X509SourceProvider == nil {
		return nil, fmt.Errorf("X509SourceProvider is required")
	}
	if cfg.ServerAuthorizer == nil {
		return nil, fmt.Errorf("server authorizer is required (e.g., tlsconfig.AuthorizeMemberOf)")
	}

	// Validate and apply defaults for timeouts
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Transport.MaxIdleConns <= 0 {
		cfg.Transport.MaxIdleConns = 100
	}
	if cfg.Transport.MaxIdleConnsPerHost <= 0 {
		cfg.Transport.MaxIdleConnsPerHost = 10
	}
	if cfg.Transport.IdleConnTimeout <= 0 {
		cfg.Transport.IdleConnTimeout = 90 * time.Second
	}

	// Get X509Source from provider (production or in-memory)
	svidSource, bundleSource, closer, err := cfg.X509SourceProvider.GetX509Source(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get X509Source: %w", err)
	}

	// Create mTLS client configuration
	// - Client presents its SVID to server
	// - Server must present valid SVID matching authorizer
	tlsConfig := tlsconfig.MTLSClientConfig(
		svidSource,           // SVID source (client certificate)
		bundleSource,         // Bundle source (trusted CAs)
		cfg.ServerAuthorizer, // Server identity verification
	)

	// Optional hardening: enforce minimum TLS version (uncomment for strict org policy)
	// tlsConfig.MinVersion = tls.VersionTLS12

	// Create HTTP transport with mTLS and resilience tuning
	transport := &http.Transport{
		TLSClientConfig:       tlsConfig,
		MaxIdleConns:          cfg.Transport.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.Transport.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.Transport.IdleConnTimeout,
		TLSHandshakeTimeout:   10 * time.Second, // Prevent hanging on slow TLS handshakes
		ExpectContinueTimeout: 1 * time.Second,  // Timeout for 100-Continue response
		// HTTP/2 enabled by default; MaxConnsPerHost can be set if needed for rate limiting
	}

	// Create HTTP client with configured timeout
	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}

	return &SPIFFEHTTPClient{
		client: client,
		closer: closer,
	}, nil
}

// ensureHTTPS validates that the URL uses HTTPS protocol.
// mTLS only applies to HTTPS requests; HTTP requests will be sent in plaintext without client certificates.
func ensureHTTPS(url string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("mTLS client requires https URL: got %q", url)
	}
	return nil
}

// do is a DRY helper for creating and executing HTTP requests.
func (c *SPIFFEHTTPClient) do(ctx context.Context, method, url, contentType string, body io.Reader) (*http.Response, error) {
	// Enforce HTTPS for mTLS security
	if err := ensureHTTPS(url); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", method, err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return c.client.Do(req)
}

// Get performs an HTTPS GET request.
func (c *SPIFFEHTTPClient) Get(ctx context.Context, url string) (*http.Response, error) {
	return c.do(ctx, http.MethodGet, url, "", nil)
}

// Post performs an HTTPS POST request.
func (c *SPIFFEHTTPClient) Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	return c.do(ctx, http.MethodPost, url, contentType, body)
}

// Put performs an HTTPS PUT request.
func (c *SPIFFEHTTPClient) Put(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	return c.do(ctx, http.MethodPut, url, contentType, body)
}

// Delete performs an HTTPS DELETE request.
func (c *SPIFFEHTTPClient) Delete(ctx context.Context, url string) (*http.Response, error) {
	return c.do(ctx, http.MethodDelete, url, "", nil)
}

// Patch performs an HTTPS PATCH request.
func (c *SPIFFEHTTPClient) Patch(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	return c.do(ctx, http.MethodPatch, url, contentType, body)
}

// Do performs an HTTP request.
func (c *SPIFFEHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

// Close releases all resources used by the client.
// Closes the X509Source first to stop new TLS handshakes, then closes idle connections.
func (c *SPIFFEHTTPClient) Close() error {
	// Close X509Source first (stops SVID fetching and rotation, prevents new TLS handshakes)
	if c.closer != nil {
		if err := c.closer.Close(); err != nil {
			return fmt.Errorf("close X509Source: %w", err)
		}
	}

	// Then close idle connections to clean up resources
	if c.client != nil {
		c.client.CloseIdleConnections()
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
