package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// Client is an HTTP client that uses X.509 SVIDs for mTLS authentication.
// The client automatically presents its SVID to servers and verifies server identity.
type Client interface {
	// Get performs an HTTP GET request.
	Get(ctx context.Context, url string) (*http.Response, error)

	// Post performs an HTTP POST request.
	Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error)

	// Do performs an HTTP request.
	Do(req *http.Request) (*http.Response, error)

	// Close releases all resources used by the client.
	Close() error
}

// Config contains configuration for creating an HTTP client.
type Config struct {
	// WorkloadAPI configuration for connecting to SPIRE agent
	WorkloadAPI WorkloadAPIConfig

	// SPIFFE configuration for server authentication
	SPIFFE SPIFFEConfig

	// HTTP client configuration
	HTTP HTTPClientConfig
}

// WorkloadAPIConfig configures the connection to the SPIRE Workload API.
type WorkloadAPIConfig struct {
	// SocketPath is the path to the SPIRE agent's Unix domain socket.
	// Example: "unix:///tmp/spire-agent/public/api.sock"
	SocketPath string
}

// SPIFFEConfig configures server authentication using SPIFFE IDs.
type SPIFFEConfig struct {
	// ExpectedServerID is the SPIFFE ID that the server must present.
	// If empty, any server from the trust domain is accepted.
	// Example: "spiffe://example.org/server"
	ExpectedServerID string

	// ExpectedTrustDomain restricts servers to a specific trust domain.
	// If empty, uses the client's trust domain.
	// Example: "example.org"
	ExpectedTrustDomain string
}

// HTTPClientConfig configures HTTP client behavior.
type HTTPClientConfig struct {
	// Timeout is the maximum time for the entire request/response cycle.
	Timeout time.Duration

	// MaxIdleConns controls the maximum number of idle connections.
	MaxIdleConns int

	// MaxIdleConnsPerHost controls the maximum idle connections per host.
	MaxIdleConnsPerHost int

	// IdleConnTimeout is the maximum time an idle connection remains open.
	IdleConnTimeout time.Duration
}

// spiffeClient is the concrete implementation using go-spiffe SDK.
type spiffeClient struct {
	config     Config
	x509Source *workloadapi.X509Source
	httpClient *http.Client
}

// New creates a new HTTP client that uses X.509 SVIDs for mTLS.
// The client fetches its SVID from the SPIRE agent and uses it for authentication.
// Server authentication is configured based on the Config.SPIFFE settings.
func New(ctx context.Context, cfg Config) (Client, error) {
	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create X.509 source from SPIRE Workload API
	// This handles automatic SVID fetching and rotation
	x509Source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(
			workloadapi.WithAddr(cfg.WorkloadAPI.SocketPath),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create X509Source: %w", err)
	}

	// Create authorizer for server identity verification
	authorizer, err := createAuthorizer(cfg.SPIFFE)
	if err != nil {
		x509Source.Close()
		return nil, fmt.Errorf("failed to create authorizer: %w", err)
	}

	// Create mTLS client configuration
	// - Client presents its SVID to server
	// - Server must present valid SVID matching authorizer
	tlsConfig := tlsconfig.MTLSClientConfig(
		x509Source, // SVID source (client certificate)
		x509Source, // Bundle source (trusted CAs)
		authorizer, // Server identity verification
	)

	// Create HTTP transport with mTLS
	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        cfg.HTTP.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.HTTP.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.HTTP.IdleConnTimeout,
	}

	// Create HTTP client
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   cfg.HTTP.Timeout,
	}

	return &spiffeClient{
		config:     cfg,
		x509Source: x509Source,
		httpClient: httpClient,
	}, nil
}

// Get performs an HTTP GET request.
func (c *spiffeClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	return c.httpClient.Do(req)
}

// Post performs an HTTP POST request.
func (c *spiffeClient) Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	return c.httpClient.Do(req)
}

// Do performs an HTTP request.
func (c *spiffeClient) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// Close releases all resources used by the client.
func (c *spiffeClient) Close() error {
	// Close HTTP client connections
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}

	// Close X509Source (stops SVID fetching and rotation)
	if c.x509Source != nil {
		if err := c.x509Source.Close(); err != nil {
			return fmt.Errorf("failed to close X509Source: %w", err)
		}
	}

	return nil
}

// DefaultConfig returns a configuration with reasonable defaults.
func DefaultConfig() Config {
	return Config{
		WorkloadAPI: WorkloadAPIConfig{
			SocketPath: "unix:///tmp/spire-agent/public/api.sock",
		},
		SPIFFE: SPIFFEConfig{
			ExpectedServerID:    "", // Allow any server from trust domain
			ExpectedTrustDomain: "",
		},
		HTTP: HTTPClientConfig{
			Timeout:             30 * time.Second,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// validateConfig validates the client configuration.
func validateConfig(cfg Config) error {
	if cfg.WorkloadAPI.SocketPath == "" {
		return fmt.Errorf("WorkloadAPI.SocketPath is required")
	}

	if cfg.HTTP.Timeout == 0 {
		return fmt.Errorf("HTTP.Timeout must be > 0")
	}

	return nil
}

// createAuthorizer creates a go-spiffe authorizer for server verification.
func createAuthorizer(cfg SPIFFEConfig) (tlsconfig.Authorizer, error) {
	// Case 1: Specific server SPIFFE ID required
	if cfg.ExpectedServerID != "" {
		serverID, err := spiffeid.FromString(cfg.ExpectedServerID)
		if err != nil {
			return nil, fmt.Errorf("invalid ExpectedServerID: %w", err)
		}
		return tlsconfig.AuthorizeID(serverID), nil
	}

	// Case 2: Specific trust domain required
	if cfg.ExpectedTrustDomain != "" {
		trustDomain, err := spiffeid.TrustDomainFromString(cfg.ExpectedTrustDomain)
		if err != nil {
			return nil, fmt.Errorf("invalid ExpectedTrustDomain: %w", err)
		}
		return tlsconfig.AuthorizeMemberOf(trustDomain), nil
	}

	// Case 3: Any authenticated server (same trust domain as client)
	// This is secure because mTLS ensures server has valid SVID from SPIRE
	return tlsconfig.AuthorizeAny(), nil
}
