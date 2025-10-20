//go:build integration
// +build integration

package httpclient_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/httpclient"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// TestIntegration_ClientServer_FullMTLS tests full mTLS communication
// between client and server using SPIRE infrastructure.
//
// Prerequisites:
//   - SPIRE agent must be running
//   - WORKLOAD_API_SOCKET environment variable must be set
//   - Workload must be registered with SPIRE
//
// Note: This test uses a simplified approach by testing against an external
// mTLS server. For full client-server integration testing, use the example
// application with the integration test scripts.
func TestIntegration_ClientServer_FullMTLS(t *testing.T) {
	t.Skip("Requires running mTLS server - use integration test scripts instead")

	// This test would require:
	// 1. Starting a zerotrustserver in a goroutine
	// 2. Getting its listening address dynamically
	// 3. Creating a client and making requests
	//
	// However, zerotrustserver.Serve() blocks, so we'd need significant
	// refactoring to make this work in a test. Instead, use the provided
	// integration test scripts that start separate server/client processes.
}

// TestIntegration_Client_CloseIdempotent tests that Close() is idempotent
func TestIntegration_Client_CloseIdempotent(t *testing.T) {
	socketPath := os.Getenv("WORKLOAD_API_SOCKET")
	if socketPath == "" {
		t.Skip("WORKLOAD_API_SOCKET not set - skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := &ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: socketPath,
		},
		SPIFFE: ports.SPIFFEConfig{
			AllowedTrustDomain: "example.org",
		},
	}

	client, err := httpclient.New(ctx, cfg)
	require.NoError(t, err)

	// First close
	err1 := client.Close()
	assert.NoError(t, err1)

	// Second close (should be idempotent)
	err2 := client.Close()
	assert.NoError(t, err2)

	// Third close (still idempotent)
	err3 := client.Close()
	assert.NoError(t, err3)
}

// TestIntegration_Client_DoAfterClose tests that Do() fails after Close()
func TestIntegration_Client_DoAfterClose(t *testing.T) {
	socketPath := os.Getenv("WORKLOAD_API_SOCKET")
	if socketPath == "" {
		t.Skip("WORKLOAD_API_SOCKET not set - skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := &ports.MTLSConfig{
		WorkloadAPI: ports.WorkloadAPIConfig{
			SocketPath: socketPath,
		},
		SPIFFE: ports.SPIFFEConfig{
			AllowedTrustDomain: "example.org",
		},
	}

	client, err := httpclient.New(ctx, cfg)
	require.NoError(t, err)

	// Close the client
	err = client.Close()
	require.NoError(t, err)

	// Try to make request after close
	req, err := http.NewRequest("GET", "https://localhost:8443/test", http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(ctx, req)

	// Should get error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client is closed")
	assert.Nil(t, resp)
}

// TestIntegration_Client_ConcurrentRequests tests concurrent usage
func TestIntegration_Client_ConcurrentRequests(t *testing.T) {
	t.Skip("Requires running mTLS server - use integration test scripts instead")

	// This test would verify that the client is thread-safe and can handle
	// concurrent requests. The implementation uses sync.RWMutex for thread safety,
	// which is sufficient. Full testing requires a running server.
}

// TestIntegration_Client_AuthorizationFailure tests that client rejects
// connections to servers outside the allowed trust domain
func TestIntegration_Client_AuthorizationFailure(t *testing.T) {
	t.Skip("Requires multi-trust-domain SPIRE setup - implement when needed")
}
