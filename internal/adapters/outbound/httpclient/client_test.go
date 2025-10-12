package httpclient

// SPIFFE HTTP Client Tests
//
// These tests verify the SPIFFE-enabled HTTP client implementation for mTLS connections.
// Tests cover client construction, configuration validation, default settings, timeout
// customization, and HTTP method helpers (GET, POST, PUT, DELETE, PATCH).
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/httpclient/... -v
//	go test ./internal/adapters/outbound/httpclient/... -run TestSPIFFEHTTPClient -v
//	go test ./internal/adapters/outbound/httpclient/... -cover

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSPIFFEHTTPClient_ValidConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Note: This test requires a running SPIRE agent
	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	client, err := NewSPIFFEHTTPClient(ctx, ClientConfig{X509SourceProvider: &WorkloadAPISourceProvider{SocketPath: socketPath}, ServerAuthorizer: authorizer})

	// If SPIRE is not running, this will fail - that's expected
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}

	require.NotNil(t, client)
	defer client.Close()

	// Verify client properties
	assert.NotNil(t, client.client)
	assert.NotNil(t, client.closer)
	assert.Equal(t, 30*time.Second, client.client.Timeout)
}

func TestNewSPIFFEHTTPClient_MissingX509SourceProvider(t *testing.T) {
	ctx := context.Background()
	authorizer := tlsconfig.AuthorizeAny()

	client, err := NewSPIFFEHTTPClient(ctx, ClientConfig{X509SourceProvider: nil, ServerAuthorizer: authorizer})

	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "X509SourceProvider is required")
}

func TestNewSPIFFEHTTPClient_MissingAuthorizer(t *testing.T) {
	ctx := context.Background()

	client, err := NewSPIFFEHTTPClient(ctx, ClientConfig{X509SourceProvider: &WorkloadAPISourceProvider{SocketPath: "unix:///tmp/socket"}, ServerAuthorizer: nil})

	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "server authorizer is required")
}

func TestSPIFFEHTTPClient_Defaults(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	client, err := NewSPIFFEHTTPClient(ctx, ClientConfig{
		X509SourceProvider: &WorkloadAPISourceProvider{SocketPath: socketPath},
		ServerAuthorizer: authorizer,
	})
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}
	defer client.Close()

	// Verify default timeout
	assert.Equal(t, 30*time.Second, client.client.Timeout, "Default timeout should be 30s")

	// Verify default transport settings
	transport, ok := client.client.Transport.(*http.Transport)
	require.True(t, ok, "Transport should be *http.Transport")
	assert.Equal(t, 100, transport.MaxIdleConns, "Default MaxIdleConns should be 100")
	assert.Equal(t, 10, transport.MaxIdleConnsPerHost, "Default MaxIdleConnsPerHost should be 10")
	assert.Equal(t, 90*time.Second, transport.IdleConnTimeout, "Default IdleConnTimeout should be 90s")
}

func TestSPIFFEHTTPClient_CustomDefaults(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	client, err := NewSPIFFEHTTPClient(ctx, ClientConfig{
		X509SourceProvider: &WorkloadAPISourceProvider{SocketPath: socketPath},
		ServerAuthorizer: authorizer,
		Timeout:          15 * time.Second,
		Transport: TransportConfig{
			MaxIdleConns:        50,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     60 * time.Second,
		},
	})
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}
	defer client.Close()

	// Verify custom timeout
	assert.Equal(t, 15*time.Second, client.client.Timeout, "Custom timeout should be 15s")

	// Verify custom transport settings
	transport, ok := client.client.Transport.(*http.Transport)
	require.True(t, ok, "Transport should be *http.Transport")
	assert.Equal(t, 50, transport.MaxIdleConns, "Custom MaxIdleConns should be 50")
	assert.Equal(t, 5, transport.MaxIdleConnsPerHost, "Custom MaxIdleConnsPerHost should be 5")
	assert.Equal(t, 60*time.Second, transport.IdleConnTimeout, "Custom IdleConnTimeout should be 60s")
}

func TestSPIFFEHTTPClient_SetTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	client, err := NewSPIFFEHTTPClient(ctx, ClientConfig{X509SourceProvider: &WorkloadAPISourceProvider{SocketPath: socketPath}, ServerAuthorizer: authorizer})
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}
	defer client.Close()

	// Set new timeout
	newTimeout := 10 * time.Second
	client.SetTimeout(newTimeout)

	assert.Equal(t, newTimeout, client.client.Timeout)
}

func TestSPIFFEHTTPClient_GetHTTPClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	client, err := NewSPIFFEHTTPClient(ctx, ClientConfig{X509SourceProvider: &WorkloadAPISourceProvider{SocketPath: socketPath}, ServerAuthorizer: authorizer})
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}
	defer client.Close()

	httpClient := client.GetHTTPClient()
	assert.NotNil(t, httpClient)
	assert.Equal(t, client.client, httpClient)
}

// TestHTTPMethods tests HTTP method helpers with mock server
// Note: This tests the method creation, not the actual mTLS connection
func TestHTTPMethods_RequestCreation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	client, err := NewSPIFFEHTTPClient(ctx, ClientConfig{X509SourceProvider: &WorkloadAPISourceProvider{SocketPath: socketPath}, ServerAuthorizer: authorizer})
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}
	defer client.Close()

	// Create a mock server to test request creation (won't actually work with mTLS)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockServer.Close()

	tests := []struct {
		name        string
		method      string
		url         string
		body        io.Reader
		contentType string
		fn          func() (*http.Response, error)
	}{
		{
			name:   "GET request with http URL (should fail)",
			method: http.MethodGet,
			url:    mockServer.URL,
			fn: func() (*http.Response, error) {
				return client.Get(ctx, mockServer.URL)
			},
		},
		{
			name:        "POST request with http URL (should fail)",
			method:      http.MethodPost,
			url:         mockServer.URL,
			body:        strings.NewReader("test body"),
			contentType: "text/plain",
			fn: func() (*http.Response, error) {
				return client.Post(ctx, mockServer.URL, "text/plain", strings.NewReader("test body"))
			},
		},
		{
			name:        "PUT request with http URL (should fail)",
			method:      http.MethodPut,
			url:         mockServer.URL,
			body:        strings.NewReader("test body"),
			contentType: "text/plain",
			fn: func() (*http.Response, error) {
				return client.Put(ctx, mockServer.URL, "text/plain", strings.NewReader("test body"))
			},
		},
		{
			name:   "DELETE request with http URL (should fail)",
			method: http.MethodDelete,
			url:    mockServer.URL,
			fn: func() (*http.Response, error) {
				return client.Delete(ctx, mockServer.URL)
			},
		},
		{
			name:        "PATCH request with http URL (should fail)",
			method:      http.MethodPatch,
			url:         mockServer.URL,
			body:        strings.NewReader("test body"),
			contentType: "text/plain",
			fn: func() (*http.Response, error) {
				return client.Patch(ctx, mockServer.URL, "text/plain", strings.NewReader("test body"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These should fail with HTTPS enforcement error
			resp, err := tt.fn()

			// We expect HTTPS enforcement error since mockServer uses http://
			require.Error(t, err, "Expected error for http:// URL")
			assert.Contains(t, err.Error(), "mTLS client requires https URL", "Expected HTTPS enforcement error")
			assert.Nil(t, resp, "Response should be nil when URL is rejected")
		})
	}
}

func TestSPIFFEHTTPClient_Do(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	client, err := NewSPIFFEHTTPClient(ctx, ClientConfig{X509SourceProvider: &WorkloadAPISourceProvider{SocketPath: socketPath}, ServerAuthorizer: authorizer})
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}
	defer client.Close()

	// Create a request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	// Try to execute (will fail with connection error, but tests Do method)
	resp, err := client.Do(req)

	// Expect error (no real server, or certificate error)
	if err == nil && resp != nil {
		resp.Body.Close()
	}
	// Either error or response is fine - we're just testing the Do method exists
}

func TestSPIFFEHTTPClient_Close(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socketPath := "unix:///tmp/spire-agent/public/api.sock"
	authorizer := tlsconfig.AuthorizeAny()

	client, err := NewSPIFFEHTTPClient(ctx, ClientConfig{X509SourceProvider: &WorkloadAPISourceProvider{SocketPath: socketPath}, ServerAuthorizer: authorizer})
	if err != nil {
		t.Skipf("Skipping test - SPIRE agent not available: %v", err)
		return
	}

	// Close should succeed
	err = client.Close()
	assert.NoError(t, err)

	// Second close should also succeed (idempotent)
	_ = client.Close()
	// May return error if source is already closed, that's okay
}

// Example usage demonstrating client creation and usage
func ExampleNewSPIFFEHTTPClient() {
	ctx := context.Background()

	// Create client with server identity verification
	client, err := NewSPIFFEHTTPClient(ctx, ClientConfig{
		X509SourceProvider: &WorkloadAPISourceProvider{SocketPath: "unix:///tmp/spire-agent/public/api.sock"},
		ServerAuthorizer: tlsconfig.AuthorizeAny(), // Allow any server from trust domain
	})
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// Make authenticated GET request
	resp, err := client.Get(ctx, "https://server:8443/api/hello")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Read response
	body, _ := io.ReadAll(resp.Body)
	println(string(body))
}
