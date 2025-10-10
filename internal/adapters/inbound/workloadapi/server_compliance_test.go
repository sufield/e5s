package workloadapi_test

// Workload API Server Compliance Tests
//
// These tests verify interface compliance and concurrency behavior.
// Tests cover ports.WorkloadAPIServer implementation and concurrent request handling.
//
// Run these tests with:
//
//	go test ./internal/adapters/inbound/workloadapi/... -v -run TestServer_ImplementsPort
//	go test ./internal/adapters/inbound/workloadapi/... -v -run TestServer_ConcurrentRequests
//	go test ./internal/adapters/inbound/workloadapi/... -cover

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/inbound/workloadapi"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/app"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer_ConcurrentRequests tests handling multiple concurrent requests
func TestServer_ConcurrentRequests(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	loader := inmemory.NewInMemoryConfig()
	factory := compose.NewInMemoryAdapterFactory()
	application, err := app.Bootstrap(ctx, loader, factory)
	require.NoError(t, err)

	socketPath := filepath.Join(t.TempDir(), "test.sock")
	server := workloadapi.NewServer(application.IdentityClientService, socketPath)

	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	time.Sleep(100 * time.Millisecond)

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
	}

	// Send multiple concurrent requests
	const numRequests = 10
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(uid int) {
			httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/svid/x509", nil)
			httpReq.Header.Set("X-Spire-Caller-UID", fmt.Sprintf("%d", 1001+uid%2))
			httpReq.Header.Set("X-Spire-Caller-PID", fmt.Sprintf("%d", 12345+uid))
			httpReq.Header.Set("X-Spire-Caller-GID", "1001")
			httpReq.Header.Set("X-Spire-Caller-Path", "/usr/bin/test")

			resp, err := client.Do(httpReq)
			if err != nil {
				results <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
				return
			}

			results <- nil
		}(i)
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

// TestServer_ImplementsPort verifies Server implements the ports.WorkloadAPIServer interface
func TestServer_ImplementsPort(t *testing.T) {
	t.Parallel()

	server := workloadapi.NewServer(nil, "/tmp/test.sock")
	var _ ports.WorkloadAPIServer = server
}
