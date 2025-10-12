package workloadapi_test

// Workload API Client Compliance Tests
//
// These tests verify client constructor, interface compliance, and concurrency behavior.
// Tests cover ports.WorkloadAPIClient implementation and concurrent request handling.
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/workloadapi/... -v -run TestClient_NewClient
//	go test ./internal/adapters/outbound/workloadapi/... -v -run TestClient_ImplementsPort
//	go test ./internal/adapters/outbound/workloadapi/... -v -run TestClient_ConcurrentRequests
//	go test ./internal/adapters/outbound/workloadapi/... -cover

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	wlapi "github.com/pocket/hexagon/spire/internal/adapters/outbound/workloadapi"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_NewClient tests constructor
func TestClient_NewClient(t *testing.T) {
	t.Parallel()

	socketPath := "/tmp/test.sock"
	client, err := wlapi.NewClient(socketPath, nil)
	require.NoError(t, err)

	assert.NotNil(t, client)
}

// TestClient_ImplementsPort verifies Client implements WorkloadAPIClient interface
func TestClient_ImplementsPort(t *testing.T) {
	t.Parallel()

	client, err := wlapi.NewClient("/tmp/test.sock", nil)
	require.NoError(t, err)
	var _ ports.WorkloadAPIClient = client
}

// TestX509SVIDResponse_ImplementsPort verifies response implements interface
func TestX509SVIDResponse_ImplementsPort(t *testing.T) {
	t.Parallel()

	resp := &wlapi.X509SVIDResponse{}
	var _ ports.X509SVIDResponse = resp
}

// TestClient_ConcurrentRequests tests concurrent request handling
func TestClient_ConcurrentRequests(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "test-concurrent.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	defer listener.Close()

	// Create test server
	ts := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			spiffeID := wlapi.SpiffePrefix + "example.org/workload"
			expiresAt := time.Now().Add(24 * time.Hour)
			certPEM, expiresTS, err := generateTestSPIFFECert(spiffeID, expiresAt)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

			resp := wlapi.X509SVIDResponse{
				SPIFFEID:  spiffeID,
				X509SVID:  certPEM,
				ExpiresAt: expiresTS,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}),
	}
	go ts.Serve(listener)
	defer ts.Close()

	time.Sleep(50 * time.Millisecond)

	client, err := wlapi.NewClient(socketPath, nil)
	require.NoError(t, err)

	// Send multiple concurrent requests
	const numRequests = 20
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			ctx := context.Background()
			_, err := client.FetchX509SVID(ctx)
			results <- err
		}()
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		select {
		case err := <-results:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent request timed out")
		}
	}
}
