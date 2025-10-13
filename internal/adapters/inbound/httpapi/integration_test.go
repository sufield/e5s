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

	"github.com/pocket/hexagon/spire/examples/httpclient"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create server with AuthorizeAny (allows any authenticated client from trust domain)
	serverAddr := ":18445"
	server, err := NewHTTPServer(ctx, ServerConfig{
		Address:    serverAddr,
		SocketPath: socketPath,
		Authorizer: tlsconfig.AuthorizeAny(),
	})
	require.NoError(t, err, "Failed to create server")
	defer server.Stop(ctx)

	// Register test handler
	server.RegisterHandler("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID, ok := GetSPIFFEID(r)
		require.True(t, ok)
		fmt.Fprintf(w, "client: %s", clientID)
	}))

	// Start server
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, server.Shutdown(shutdownCtx))
	}()

	// Wait for server to be ready
	time.Sleep(500 * time.Millisecond)

	// Create client
	client, err := httpclient.New(ctx, httpclient.Config{
		WorkloadAPI: httpclient.WorkloadAPIConfig{
			SocketPath: socketPath,
		},
		SPIFFE: httpclient.SPIFFEConfig{
			ExpectedServerID: "", // Accept any server
		},
		HTTP: httpclient.HTTPClientConfig{
			Timeout:      5 * time.Second,
			MaxIdleConns: 10,
		},
	})
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	// Test request
	resp, err := client.Get(ctx, "https://localhost"+serverAddr+"/test")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "client: spiffe://")
}

// TestMTLSClientServer_AuthorizationFailure tests that wrong client ID is rejected
func TestMTLSClientServer_AuthorizationFailure(t *testing.T) {
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	specificClientID := spiffeid.RequireFromString("spiffe://example.org/nonexistent/client")
	serverAddr := ":18446"
	server, err := NewHTTPServer(ctx, ServerConfig{
		Address:    serverAddr,
		SocketPath: socketPath,
		Authorizer: tlsconfig.AuthorizeID(specificClientID), // Only allow specific ID
	})
	require.NoError(t, err, "Failed to create server")
	defer server.Stop(ctx)

	server.RegisterHandler("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, server.Shutdown(shutdownCtx))
	}()

	time.Sleep(500 * time.Millisecond)

	// Create client (will fail auth since ID mismatch)
	client, err := httpclient.New(ctx, httpclient.Config{
		WorkloadAPI: httpclient.WorkloadAPIConfig{
			SocketPath: socketPath,
		},
		SPIFFE: httpclient.SPIFFEConfig{
			ExpectedServerID: "", // Accept any server
		},
		HTTP: httpclient.HTTPClientConfig{
			Timeout: 5 * time.Second,
		},
	})
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	// Expect auth failure
	_, err = client.Get(ctx, "https://localhost"+serverAddr+"/test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad certificate", "Should fail with TLS auth error")
}

// TestMTLSServer_HealthCheck tests basic health check endpoint
func TestMTLSServer_HealthCheck(t *testing.T) {
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create server
	serverAddr := ":18447"
	server, err := NewHTTPServer(ctx, ServerConfig{
		Address:    serverAddr,
		SocketPath: socketPath,
		Authorizer: tlsconfig.AuthorizeAny(),
	})
	require.NoError(t, err, "Failed to create server")
	defer server.Stop(ctx)

	server.RegisterHandler("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, server.Shutdown(shutdownCtx))
	}()

	time.Sleep(500 * time.Millisecond)

	// Test health endpoint
	client, err := httpclient.New(ctx, httpclient.Config{
		WorkloadAPI: httpclient.WorkloadAPIConfig{
			SocketPath: socketPath,
		},
		SPIFFE: httpclient.SPIFFEConfig{
			ExpectedServerID: "", // Accept any server
		},
		HTTP: httpclient.HTTPClientConfig{
			Timeout: 5 * time.Second,
		},
	})
	require.NoError(t, err, "Failed to create client")
	defer client.Close()

	resp, err := client.Get(ctx, "https://localhost"+serverAddr+"/health")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "OK", string(body))
}
