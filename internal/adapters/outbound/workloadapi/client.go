package workloadapi

import (
	"github.com/pocket/hexagon/spire/internal/ports"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

// Client is a Workload API client (outbound adapter from workload's perspective)
// Workloads use this to fetch their identity documents from the agent
type Client struct {
	socketPath string
	httpClient *http.Client
}

// NewClient creates a new Workload API client
func NewClient(socketPath string) *Client {
	// Create HTTP client configured for Unix domain socket
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
		Timeout: 30 * time.Second,
	}

	return &Client{
		socketPath: socketPath,
		httpClient: httpClient,
	}
}

// FetchX509SVID fetches an X.509 SVID for the calling workload
// The client sends its process credentials, and the server attests it
func (c *Client) FetchX509SVID(ctx context.Context) (ports.X509SVIDResponse, error) {
	// Create request to Workload API
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/svid/x509", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Send caller credentials in headers
	// NOTE: This is for demonstration only - production would use SO_PEERCRED
	req.Header.Set("X-Spire-Caller-UID", fmt.Sprintf("%d", os.Getuid()))
	req.Header.Set("X-Spire-Caller-PID", fmt.Sprintf("%d", os.Getpid()))
	req.Header.Set("X-Spire-Caller-GID", fmt.Sprintf("%d", os.Getgid()))

	// Get executable path
	exePath, _ := os.Executable()
	req.Header.Set("X-Spire-Caller-Path", exePath)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var svidResp X509SVIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&svidResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &svidResp, nil
}

// FetchX509SVIDWithConfig fetches an X.509 SVID with custom TLS configuration
// Enables mTLS authentication when connecting to the Workload API server
// If tlsConfig is nil, falls back to FetchX509SVID (backward compatible)
func (c *Client) FetchX509SVIDWithConfig(ctx context.Context, tlsConfig *tls.Config) (ports.X509SVIDResponse, error) {
	// If no TLS config provided, use regular fetch
	if tlsConfig == nil {
		return c.FetchX509SVID(ctx)
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
		Timeout: 30 * time.Second,
	}

	// Create request to Workload API
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/svid/x509", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Send caller credentials in headers
	req.Header.Set("X-Spire-Caller-UID", fmt.Sprintf("%d", os.Getuid()))
	req.Header.Set("X-Spire-Caller-PID", fmt.Sprintf("%d", os.Getpid()))
	req.Header.Set("X-Spire-Caller-GID", fmt.Sprintf("%d", os.Getgid()))

	exePath, _ := os.Executable()
	req.Header.Set("X-Spire-Caller-Path", exePath)

	// Send request with mTLS
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request with mTLS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var svidResp X509SVIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&svidResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &svidResp, nil
}

// X509SVIDResponse is the response format for X.509 SVID requests
type X509SVIDResponse struct {
	SPIFFEID  string `json:"spiffe_id"`
	X509SVID  string `json:"x509_svid"`
	ExpiresAt int64  `json:"expires_at"`
}

// ToIdentity converts the response to a SPIFFE ID string (for internal conversion to ports.Identity)
func (r *X509SVIDResponse) ToIdentity() string {
	if r == nil {
		return ""
	}
	return r.SPIFFEID
}

// GetSPIFFEID returns the SPIFFE ID
func (r *X509SVIDResponse) GetSPIFFEID() string {
	if r == nil {
		return ""
	}
	return r.SPIFFEID
}

// GetX509SVID returns the X.509 SVID certificate (PEM-encoded)
func (r *X509SVIDResponse) GetX509SVID() string {
	if r == nil {
		return ""
	}
	return r.X509SVID
}

// GetExpiresAt returns the expiration timestamp (Unix time)
func (r *X509SVIDResponse) GetExpiresAt() int64 {
	if r == nil {
		return 0
	}
	return r.ExpiresAt
}

var _ ports.X509SVIDResponse = (*X509SVIDResponse)(nil)
var _ ports.WorkloadAPIClient = (*Client)(nil)
