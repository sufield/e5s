package workloadapi

import (
	"context"
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
func (c *Client) FetchX509SVID(ctx context.Context) (*X509SVIDResponse, error) {
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

// X509SVIDResponse is the response format for X.509 SVID requests
type X509SVIDResponse struct {
	SPIFFEID  string `json:"spiffe_id"`
	X509SVID  string `json:"x509_svid"`
	ExpiresAt int64  `json:"expires_at"`
}

// ToIdentity converts the response to a ports.Identity (for internal use)
func (r *X509SVIDResponse) ToIdentity() string {
	return r.SPIFFEID
}
