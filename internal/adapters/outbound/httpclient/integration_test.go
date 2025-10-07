//go:build integration

package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/inbound/httpapi"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientServerMTLS tests full mTLS communication between client and server
func TestClientServerMTLS(t *testing.T) {
	// Get socket path from environment or use default
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create server
	serverAddr := ":18450"
	server, err := httpapi.NewHTTPServer(
		ctx,
		serverAddr,
		socketPath,
		tlsconfig.AuthorizeAny(),
	)
	require.NoError(t, err, "Failed to create server")
	defer server.Stop(ctx)

	// Register test handler
	server.RegisterHandler("/test", func(w http.ResponseWriter, r *http.Request) {
		clientID, ok := httpapi.GetSPIFFEID(r)
		require.True(t, ok, "Failed to get client SPIFFE ID")

		response := fmt.Sprintf("Hello %s", clientID.String())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	})

	// Start server
	err = server.Start(ctx)
	require.NoError(t, err, "Failed to start server")

	// Wait for server to be ready
	time.Sleep(500 * time.Millisecond)

	// Create client
	client, err := NewSPIFFEHTTPClient(
		ctx,
		socketPath,
		tlsconfig.AuthorizeAny(), // Accept any server from trust domain
	)
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	// Make request
	serverURL := fmt.Sprintf("https://localhost%s/test", serverAddr)
	resp, err := client.Get(ctx, serverURL)
	require.NoError(t, err, "Failed to make request")
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	// Response should contain "Hello spiffe://..."
	assert.Contains(t, string(body), "Hello spiffe://")
}

// TestClientAllHTTPMethods tests all HTTP methods with mTLS
func TestClientAllHTTPMethods(t *testing.T) {
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create server
	serverAddr := ":18451"
	server, err := httpapi.NewHTTPServer(
		ctx,
		serverAddr,
		socketPath,
		tlsconfig.AuthorizeAny(),
	)
	require.NoError(t, err, "Failed to create server")
	defer server.Stop(ctx)

	// Register handlers for different methods
	server.RegisterHandler("/api", func(w http.ResponseWriter, r *http.Request) {
		clientID, _ := httpapi.GetSPIFFEID(r)

		// Echo back method and read body if present
		var bodyContent string
		if r.Body != nil {
			bodyBytes, _ := io.ReadAll(r.Body)
			bodyContent = string(bodyBytes)
		}

		response := fmt.Sprintf("Method: %s, Client: %s, Body: %s",
			r.Method, clientID.String(), bodyContent)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	})

	// Start server
	err = server.Start(ctx)
	require.NoError(t, err, "Failed to start server")

	// Wait for server to be ready
	time.Sleep(500 * time.Millisecond)

	// Create client
	client, err := NewSPIFFEHTTPClient(ctx, socketPath, tlsconfig.AuthorizeAny())
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	serverURL := fmt.Sprintf("https://localhost%s/api", serverAddr)

	tests := []struct {
		name           string
		method         string
		fn             func() (*http.Response, error)
		expectedBody   string
		expectedStatus int
	}{
		{
			name:   "GET",
			method: http.MethodGet,
			fn: func() (*http.Response, error) {
				return client.Get(ctx, serverURL)
			},
			expectedBody:   "Method: GET",
			expectedStatus: http.StatusOK,
		},
		{
			name:   "POST",
			method: http.MethodPost,
			fn: func() (*http.Response, error) {
				return client.Post(ctx, serverURL, "text/plain", strings.NewReader("POST data"))
			},
			expectedBody:   "Method: POST",
			expectedStatus: http.StatusOK,
		},
		{
			name:   "PUT",
			method: http.MethodPut,
			fn: func() (*http.Response, error) {
				return client.Put(ctx, serverURL, "text/plain", strings.NewReader("PUT data"))
			},
			expectedBody:   "Method: PUT",
			expectedStatus: http.StatusOK,
		},
		{
			name:   "DELETE",
			method: http.MethodDelete,
			fn: func() (*http.Response, error) {
				return client.Delete(ctx, serverURL)
			},
			expectedBody:   "Method: DELETE",
			expectedStatus: http.StatusOK,
		},
		{
			name:   "PATCH",
			method: http.MethodPatch,
			fn: func() (*http.Response, error) {
				return client.Patch(ctx, serverURL, "text/plain", strings.NewReader("PATCH data"))
			},
			expectedBody:   "Method: PATCH",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.fn()
			require.NoError(t, err, "Failed to make %s request", tt.method)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err, "Failed to read response body")

			assert.Contains(t, string(body), tt.expectedBody)
			assert.Contains(t, string(body), "Client: spiffe://")
		})
	}
}

// TestClientServerIDVerification tests that client verifies server identity
func TestClientServerIDVerification(t *testing.T) {
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create server
	serverAddr := ":18452"
	server, err := httpapi.NewHTTPServer(
		ctx,
		serverAddr,
		socketPath,
		tlsconfig.AuthorizeAny(),
	)
	require.NoError(t, err, "Failed to create server")
	defer server.Stop(ctx)

	server.RegisterHandler("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	err = server.Start(ctx)
	require.NoError(t, err, "Failed to start server")

	time.Sleep(500 * time.Millisecond)

	// Create client that expects a specific server ID that doesn't match
	nonexistentServerID := spiffeid.RequireFromString("spiffe://example.org/nonexistent/server")
	client, err := NewSPIFFEHTTPClient(
		ctx,
		socketPath,
		tlsconfig.AuthorizeID(nonexistentServerID),
	)
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	// Make request - should fail with TLS error
	serverURL := fmt.Sprintf("https://localhost%s/test", serverAddr)
	resp, err := client.Get(ctx, serverURL)

	// Should get TLS handshake error
	assert.Error(t, err, "Expected TLS handshake to fail")
	if resp != nil {
		resp.Body.Close()
	}
}

// TestClientTimeout tests client timeout configuration
func TestClientTimeout(t *testing.T) {
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create server with slow handler
	serverAddr := ":18453"
	server, err := httpapi.NewHTTPServer(
		ctx,
		serverAddr,
		socketPath,
		tlsconfig.AuthorizeAny(),
	)
	require.NoError(t, err, "Failed to create server")
	defer server.Stop(ctx)

	server.RegisterHandler("/slow", func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than client timeout
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	err = server.Start(ctx)
	require.NoError(t, err, "Failed to start server")

	time.Sleep(500 * time.Millisecond)

	// Create client with short timeout
	client, err := NewSPIFFEHTTPClient(ctx, socketPath, tlsconfig.AuthorizeAny())
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	// Set short timeout
	client.SetTimeout(1 * time.Second)

	// Make request - should timeout
	serverURL := fmt.Sprintf("https://localhost%s/slow", serverAddr)
	resp, err := client.Get(ctx, serverURL)

	// Should get timeout error
	assert.Error(t, err, "Expected timeout error")
	if resp != nil {
		resp.Body.Close()
	}
}
