package workloadapi_test

// Workload API Server Handler Tests
//
// These tests verify HTTP request handling including SVID fetch operations,
// method validation, workload registration checks, and error scenarios.
//
// Run these tests with:
//
//	go test ./internal/adapters/inbound/workloadapi/... -v -run TestServer_HandleFetchX509SVID
//	go test ./internal/adapters/inbound/workloadapi/... -cover

import (
	"context"
	"encoding/json"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer_HandleFetchX509SVID_Success tests successful SVID fetch
func TestServer_HandleFetchX509SVID_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Bootstrap application
	loader := inmemory.NewInMemoryConfig()
	factory := compose.NewInMemoryAdapterFactory()
	application, err := app.Bootstrap(ctx, loader, factory)
	require.NoError(t, err)

	socketPath := filepath.Join(t.TempDir(), "test.sock")
	server := workloadapi.NewServer(application.IdentityClientService, socketPath)

	// Test via the actual HTTP server
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create HTTP client for Unix socket
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/svid/x509", nil)
	require.NoError(t, err)
	httpReq.Header.Set("X-Spire-Caller-UID", "1001")
	httpReq.Header.Set("X-Spire-Caller-PID", "12345")
	httpReq.Header.Set("X-Spire-Caller-GID", "1001")
	httpReq.Header.Set("X-Spire-Caller-Path", "/usr/bin/server")

	// Send request
	resp, err := client.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify response
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var svidResp workloadapi.X509SVIDResponse
	err = json.NewDecoder(resp.Body).Decode(&svidResp)
	require.NoError(t, err)

	assert.Contains(t, svidResp.SPIFFEID, "example.org")
	assert.NotEmpty(t, svidResp.X509SVID)
	assert.Greater(t, svidResp.ExpiresAt, time.Now().Unix())
}

// TestServer_HandleFetchX509SVID_InvalidMethod tests method validation
func TestServer_HandleFetchX509SVID_InvalidMethod(t *testing.T) {
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

	// Test POST method (should fail)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/svid/x509", nil)
	resp, err := client.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

// TestServer_HandleFetchX509SVID_UnregisteredWorkload tests unregistered workload
func TestServer_HandleFetchX509SVID_UnregisteredWorkload(t *testing.T) {
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

	// Request with unregistered UID
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/svid/x509", nil)
	httpReq.Header.Set("X-Spire-Caller-UID", "9999") // Unregistered
	httpReq.Header.Set("X-Spire-Caller-PID", "99999")
	httpReq.Header.Set("X-Spire-Caller-GID", "9999")
	httpReq.Header.Set("X-Spire-Caller-Path", "/unknown")

	resp, err := client.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// TestServer_HandleFetchX509SVID_TableDriven tests various scenarios
func TestServer_HandleFetchX509SVID_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		uid            string
		expectedStatus int
		expectSPIFFEID bool
	}{
		{
			name:           "valid server workload",
			method:         http.MethodGet,
			uid:            "1001",
			expectedStatus: http.StatusOK,
			expectSPIFFEID: true,
		},
		{
			name:           "valid client workload",
			method:         http.MethodGet,
			uid:            "1002",
			expectedStatus: http.StatusOK,
			expectSPIFFEID: true,
		},
		{
			name:           "unregistered workload",
			method:         http.MethodGet,
			uid:            "9999",
			expectedStatus: http.StatusInternalServerError,
			expectSPIFFEID: false,
		},
		{
			name:           "invalid method POST",
			method:         http.MethodPost,
			uid:            "1001",
			expectedStatus: http.StatusMethodNotAllowed,
			expectSPIFFEID: false,
		},
		{
			name:           "invalid method PUT",
			method:         http.MethodPut,
			uid:            "1001",
			expectedStatus: http.StatusMethodNotAllowed,
			expectSPIFFEID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			loader := inmemory.NewInMemoryConfig()
			factory := compose.NewInMemoryAdapterFactory()
			application, err := app.Bootstrap(ctx, loader, factory)
			require.NoError(t, err)

			// Replace spaces in test name for valid socket path
			socketPath := filepath.Join(t.TempDir(), fmt.Sprintf("test-%d.sock", time.Now().UnixNano()%10000))
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

			httpReq, _ := http.NewRequestWithContext(ctx, tt.method, "http://unix/svid/x509", nil)
			httpReq.Header.Set("X-Spire-Caller-UID", tt.uid)
			httpReq.Header.Set("X-Spire-Caller-PID", "12345")
			httpReq.Header.Set("X-Spire-Caller-GID", tt.uid)
			httpReq.Header.Set("X-Spire-Caller-Path", "/usr/bin/test")

			resp, err := client.Do(httpReq)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if tt.expectSPIFFEID {
				var svidResp workloadapi.X509SVIDResponse
				err = json.NewDecoder(resp.Body).Decode(&svidResp)
				require.NoError(t, err)
				assert.Contains(t, svidResp.SPIFFEID, "example.org")
			}
		})
	}
}
