// client.go contains the WorkloadAPIClient implementation.
//
// Package workloadapi provides a production-ready client adapter for the SPIFFE Workload API.
//
// This adapter enables workloads to fetch their identity documents (X.509 SVIDs)
// from a SPIRE Agent via Unix domain socket communication. It implements the
// ports.WorkloadAPIClient interface for hexagonal architecture compliance.
//
// Communication Protocol:
//   - HTTP over Unix domain sockets
//   - JSON request/response format
//   - Configurable timeouts and endpoints
//
// Workload Attestation (Production-Ready):
//
//	This implementation uses kernel-level credential passing for secure workload attestation.
//	The companion server extracts process credentials (PID, UID, GID) using SO_PEERCRED on Linux,
//	which provides kernel-verified identity that cannot be forged by the caller.
//
//	Platform Support:
//	  - Linux: SO_PEERCRED (fully implemented and production-ready)
//	  - Other platforms: Requires platform-specific implementation (getpeereid, getpeerucred, etc.)
//
//	Security Guarantee:
//	  Unlike header-based or other user-space attestation mechanisms, SO_PEERCRED credentials
//	  are verified by the kernel and cannot be spoofed. This provides the same security level
//	  as production SPIRE deployments.
//
// Example Usage:
//
//	client, err := workloadapi.NewClient("/tmp/spire-agent/public/api.sock", nil)
//	if err != nil {
//	    return fmt.Errorf("failed to create client: %w", err)
//	}
//
//	resp, err := client.FetchX509SVID(ctx)
//	if err != nil {
//	    return fmt.Errorf("failed to fetch SVID: %w", err)
//	}
//
//	fmt.Printf("SPIFFE ID: %s\n", resp.GetSPIFFEID())
package workloadapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// ClientOpts contains optional configuration for the Workload API client
type ClientOpts struct {
	// Timeout for HTTP requests (default: 30 seconds)
	Timeout time.Duration

	// Endpoint URL for SVID fetches (default: "http://unix/svid/x509")
	Endpoint string
}

// Client is a Workload API client (outbound adapter from workload's perspective).
//
// Workloads use this client to fetch their identity documents from the SPIRE Agent.
// Communication happens over Unix domain sockets using HTTP protocol.
//
// Thread Safety: Client is safe for concurrent use by multiple goroutines.
type Client struct {
	socketPath string
	endpoint   string
	timeout    time.Duration
	httpClient *http.Client
}

// NewClient creates a new Workload API client for Unix socket communication.
//
// The socketPath should be an absolute path to the SPIRE Agent's Workload API socket,
// typically "/tmp/spire-agent/public/api.sock" in development or a configured path
// in production.
//
// Parameters:
//   - socketPath: Absolute path to Unix domain socket (must start with '/')
//   - opts: Optional configuration (nil uses defaults)
//
// Returns:
//   - *Client: Configured client ready for SVID fetching
//   - error: Non-nil if socketPath is invalid
//
// Example:
//
//	client, err := NewClient("/tmp/spire-agent/public/api.sock", nil)
//	if err != nil {
//	    return fmt.Errorf("client creation failed: %w", err)
//	}
func NewClient(socketPath string, opts *ClientOpts) (*Client, error) {
	// Strip unix:// prefix if present (for config compatibility)
	socketPath = strings.TrimPrefix(socketPath, "unix://")

	// Validate socket path (absolute paths or Linux abstract sockets)
	if socketPath == "" || !(strings.HasPrefix(socketPath, "/") || strings.HasPrefix(socketPath, "@")) {
		return nil, fmt.Errorf("%w: got %q", ErrInvalidSocketPath, socketPath)
	}

	// Apply defaults for optional configuration
	if opts == nil {
		opts = &ClientOpts{
			Timeout:  DefaultTimeout,
			Endpoint: DefaultSVIDEndpoint,
		}
	}
	if opts.Timeout == 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.Endpoint == "" {
		opts.Endpoint = DefaultSVIDEndpoint
	}

	// Create HTTP client configured for Unix domain socket
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
		Timeout: opts.Timeout,
	}

	return &Client{
		socketPath: socketPath,
		endpoint:   opts.Endpoint,
		timeout:    opts.Timeout,
		httpClient: httpClient,
	}, nil
}

// FetchX509SVID fetches an X.509 SVID for the calling workload.
//
// Workload Attestation:
// The server automatically extracts kernel-verified process credentials (PID, UID, GID, path)
// using SO_PEERCRED on Linux. No headers or client-provided data are needed. The credentials
// are verified by the kernel and cannot be forged.
//
// Parameters:
//   - ctx: Context for request lifecycle (timeout, cancellation)
//
// Returns:
//   - ports.X509SVIDResponse: The fetched SVID with SPIFFE ID, certificate, and expiration
//   - error: Wrapped with ErrFetchFailed if request fails, ErrServerError if server error,
//     ErrInvalidResponse if response malformed
//
// Example:
//
//	resp, err := client.FetchX509SVID(ctx)
//	if errors.Is(err, workloadapi.ErrServerError) {
//	    log.Printf("server error: %v", err)
//	}
func (c *Client) FetchX509SVID(ctx context.Context) (ports.X509SVIDResponse, error) {
	// Create request - server uses SO_PEERCRED for attestation
	req, err := c.newSVIDRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}

	// Send request via HTTP over Unix socket
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}
	defer resp.Body.Close()

	// Handle non-OK status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, MaxErrorBodySize))
		return nil, fmt.Errorf("%w: status %d: %s", ErrServerError, resp.StatusCode, string(body))
	}

	// Parse and validate response (limit body size to prevent oversized payloads)
	var svidResp X509SVIDResponse
	limitedBody := io.LimitReader(resp.Body, MaxResponseBodySize)
	if err := json.NewDecoder(limitedBody).Decode(&svidResp); err != nil {
		return nil, fmt.Errorf("%w: decode failed: %v", ErrInvalidResponse, err)
	}

	// Validate response contains required fields
	if err := svidResp.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}

	return &svidResp, nil
}

// newSVIDRequest creates a new HTTP request for SVID fetching.
//
// Workload Attestation:
// The server automatically extracts the calling process's credentials (PID, UID, GID, path)
// using SO_PEERCRED on Linux. The kernel verifies these credentials at the Unix socket layer,
// making them impossible to forge. No headers or explicit attestation data are needed from the client.
//
// This is a significant security improvement over header-based attestation, which relied on
// client-provided data that could potentially be spoofed.
//
// Returns:
//   - *http.Request: Configured request for SVID fetch
//   - error: Non-nil if request creation fails
func (c *Client) newSVIDRequest(ctx context.Context) (*http.Request, error) {
	// Create base request - no attestation headers needed
	// Server extracts kernel-verified credentials automatically via SO_PEERCRED
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return req, nil
}

// FetchX509SVIDWithConfig fetches an X.509 SVID with custom TLS configuration.
//
// This method enables mTLS (mutual TLS) authentication when connecting to the
// Workload API server. The tlsConfig must include the SPIRE trust bundle as RootCAs
// for server certificate verification. For client authentication, include the
// workload's own certificate and private key in Certificates.
//
// Parameters:
//   - ctx: Context for request lifecycle (timeout, cancellation)
//   - tlsConfig: TLS configuration for mTLS (must not be nil)
//
// Returns:
//   - ports.X509SVIDResponse: The fetched SVID
//   - error: Wrapped with sentinel errors for inspectable error handling
//
// Example:
//
//	tlsConfig := &tls.Config{
//	    RootCAs:      spireBundle,           // Trust bundle for server verification
//	    Certificates: []tls.Certificate{...}, // Client cert for mTLS
//	}
//	resp, err := client.FetchX509SVIDWithConfig(ctx, tlsConfig)
func (c *Client) FetchX509SVIDWithConfig(ctx context.Context, tlsConfig *tls.Config) (ports.X509SVIDResponse, error) {
	if tlsConfig == nil {
		return nil, fmt.Errorf("%w: tlsConfig cannot be nil", ErrInvalidArgument)
	}

	// Create HTTP client with custom TLS config for mTLS
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", c.socketPath)
			},
			TLSClientConfig: tlsConfig,
		},
		Timeout: c.timeout, // Use configured timeout, not hard-coded
	}

	// Create and configure request (reuses helper to avoid duplication)
	req, err := c.newSVIDRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}

	// Send request with mTLS
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w (mTLS): %v", ErrFetchFailed, err)
	}
	defer resp.Body.Close()

	// Handle non-OK status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, MaxErrorBodySize))
		return nil, fmt.Errorf("%w (mTLS): status %d: %s", ErrServerError, resp.StatusCode, string(body))
	}

	// Parse and validate response (limit body size to prevent oversized payloads)
	var svidResp X509SVIDResponse
	limitedBody := io.LimitReader(resp.Body, MaxResponseBodySize)
	if err := json.NewDecoder(limitedBody).Decode(&svidResp); err != nil {
		return nil, fmt.Errorf("%w (mTLS): decode failed: %v", ErrInvalidResponse, err)
	}

	// Validate response contains required fields
	if err := svidResp.Validate(); err != nil {
		return nil, fmt.Errorf("%w (mTLS): %v", ErrInvalidResponse, err)
	}

	return &svidResp, nil
}

// Compile-time interface compliance verification
var (
	_ ports.WorkloadAPIClient = (*Client)(nil)
)
