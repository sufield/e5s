//go:build integration

package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/httpclient"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMTLSClientServer tests full mTLS communication between client and server
// This requires SPIRE running with workloads registered
func TestMTLSClientServer(t *testing.T) {
	// Get socket path from environment or use default
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create server with AuthorizeAny (allows any authenticated client from trust domain)
	serverAddr := ":18445"
	server, err := NewHTTPServer(
		ctx,
		serverAddr,
		socketPath,
		tlsconfig.AuthorizeAny(),
	)
	require.NoError(t, err, "Failed to create server")
	defer server.Stop(ctx)

	// Register test handler
	server.RegisterHandler("/test", func(w http.ResponseWriter, r *http.Request) {
		clientID, ok := GetSPIFFEID(r)
		require.True(t, ok, "Failed to get client SPIFFE ID")

		response := fmt.Sprintf("client: %s", clientID.String())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	})

	// Start server
	err = server.Start(ctx)
	require.NoError(t, err, "Failed to start server")

	// Wait for server to be ready
	time.Sleep(500 * time.Millisecond)

	// Create client configuration
	clientCfg := httpclient.DefaultConfig()
	clientCfg.WorkloadAPI.SocketPath = socketPath
	clientCfg.SPIFFE.ExpectedServerID = "" // Accept any server from trust domain
	clientCfg.SPIFFE.ExpectedTrustDomain = ""

	// Create client
	client, err := httpclient.New(ctx, clientCfg)
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

	// Response should contain "client: spiffe://..."
	assert.Contains(t, string(body), "client: spiffe://")
}

// TestMTLSClientServer_AuthorizationFailure tests that wrong client ID is rejected
func TestMTLSClientServer_AuthorizationFailure(t *testing.T) {
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create server that only allows a specific client SPIFFE ID
	// Use an ID that won't match the actual client
	specificClientID := spiffeid.RequireFromString("spiffe://example.org/nonexistent/client")

	serverAddr := ":18446"
	server, err := NewHTTPServer(
		ctx,
		serverAddr,
		socketPath,
		tlsconfig.AuthorizeID(specificClientID), // Only allow specific ID
	)
	require.NoError(t, err, "Failed to create server")
	defer server.Stop(ctx)

	// Register test handler
	server.RegisterHandler("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	})

	// Start server
	err = server.Start(ctx)
	require.NoError(t, err, "Failed to start server")

	// Wait for server to be ready
	time.Sleep(500 * time.Millisecond)

	// Create client configuration
	clientCfg := httpclient.DefaultConfig()
	clientCfg.WorkloadAPI.SocketPath = socketPath

	// Create client (will have different SPIFFE ID than expected)
	client, err := httpclient.New(ctx, clientCfg)
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	// Make request - should fail with TLS error
	serverURL := fmt.Sprintf("https://localhost%s/test", serverAddr)
	resp, err := client.Get(ctx, serverURL)

	// Should get error (TLS handshake failure)
	assert.Error(t, err, "Expected TLS handshake to fail")
	if resp != nil {
		resp.Body.Close()
	}
}

// TestMTLSServer_HealthCheck tests basic health check endpoint
func TestMTLSServer_HealthCheck(t *testing.T) {
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create server
	serverAddr := ":18447"
	server, err := NewHTTPServer(
		ctx,
		serverAddr,
		socketPath,
		tlsconfig.AuthorizeAny(),
	)
	require.NoError(t, err, "Failed to create server")
	defer server.Stop(ctx)

	// Register health check handler
	server.RegisterHandler("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Start server
	err = server.Start(ctx)
	require.NoError(t, err, "Failed to start server")

	// Wait for server to be ready
	time.Sleep(500 * time.Millisecond)

	// Create client
	clientCfg := httpclient.DefaultConfig()
	clientCfg.WorkloadAPI.SocketPath = socketPath

	client, err := httpclient.New(ctx, clientCfg)
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	// Make health check request
	serverURL := fmt.Sprintf("https://localhost%s/health", serverAddr)
	resp, err := client.Get(ctx, serverURL)
	require.NoError(t, err, "Failed to make request")
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")
	assert.Equal(t, "OK", string(body))
}
