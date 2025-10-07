package workloadapi_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/inbound/workloadapi"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	wlapi "github.com/pocket/hexagon/spire/internal/adapters/outbound/workloadapi"
	"github.com/pocket/hexagon/spire/internal/app"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_FetchX509SVID_Success tests successful SVID fetch
func TestClient_FetchX509SVID_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Start real server
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

	// Create client and fetch SVID
	client := wlapi.NewClient(socketPath)

	// Mock the UID to match registered workload (since we're running as current user)
	// In production, the server would extract this via SO_PEERCRED
	resp, err := client.FetchX509SVID(ctx)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify response structure
	assert.NotEmpty(t, resp.GetSPIFFEID())
	assert.NotEmpty(t, resp.GetX509SVID())
	assert.Greater(t, resp.GetExpiresAt(), int64(0))
}

// TestClient_FetchX509SVID_ServerError tests server error handling
func TestClient_FetchX509SVID_ServerError(t *testing.T) {
	t.Parallel()

	// Create mock HTTP server that returns error
	socketPath := filepath.Join(t.TempDir(), "test-error.sock")

	// Remove existing socket
	os.RemoveAll(socketPath)

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	defer listener.Close()

	// Create test server that returns errors
	ts := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
		}),
	}

	go ts.Serve(listener)
	defer ts.Close()

	time.Sleep(50 * time.Millisecond)

	// Create client
	client := wlapi.NewClient(socketPath)

	// Attempt fetch
	_, err = client.FetchX509SVID(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server returned error 500")
}

// TestClient_FetchX509SVID_InvalidResponse tests invalid JSON response handling
func TestClient_FetchX509SVID_InvalidResponse(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "test-invalid.sock")
	os.RemoveAll(socketPath)

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	defer listener.Close()

	ts := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json"))
		}),
	}

	go ts.Serve(listener)
	defer ts.Close()

	time.Sleep(50 * time.Millisecond)

	client := wlapi.NewClient(socketPath)

	_, err = client.FetchX509SVID(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
}

// TestClient_FetchX509SVID_SocketNotFound tests connection error handling
func TestClient_FetchX509SVID_SocketNotFound(t *testing.T) {
	t.Parallel()

	// Use non-existent socket path
	socketPath := "/tmp/nonexistent-spire-socket-12345.sock"

	client := wlapi.NewClient(socketPath)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := client.FetchX509SVID(ctx)
	assert.Error(t, err)
}

// TestClient_FetchX509SVIDWithConfig_Success tests mTLS fetch
func TestClient_FetchX509SVIDWithConfig_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Start real server
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

	// Create client with nil TLS config (should fallback to regular fetch)
	client := wlapi.NewClient(socketPath)
	resp, err := client.FetchX509SVIDWithConfig(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestClient_NewClient tests constructor
func TestClient_NewClient(t *testing.T) {
	t.Parallel()

	socketPath := "/tmp/test.sock"
	client := wlapi.NewClient(socketPath)

	assert.NotNil(t, client)
}

// TestClient_FetchX509SVID_TableDriven tests various scenarios
func TestClient_FetchX509SVID_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupServer   func(t *testing.T) (string, func())
		wantError     bool
		wantErrMsg string
	}{
		{
			name: "server returns 500",
			setupServer: func(t *testing.T) (string, func()) {
				socketPath := filepath.Join(t.TempDir(), "test-500.sock")
				listener, _ := net.Listen("unix", socketPath)
				ts := &http.Server{
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte("server error"))
					}),
				}
				go ts.Serve(listener)
				return socketPath, func() { ts.Close(); listener.Close() }
			},
			wantError:  true,
			wantErrMsg: "server returned error 500",
		},
		{
			name: "server returns 404",
			setupServer: func(t *testing.T) (string, func()) {
				socketPath := filepath.Join(t.TempDir(), "test-404.sock")
				listener, _ := net.Listen("unix", socketPath)
				ts := &http.Server{
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						w.Write([]byte("not found"))
					}),
				}
				go ts.Serve(listener)
				return socketPath, func() { ts.Close(); listener.Close() }
			},
			wantError:  true,
			wantErrMsg: "server returned error 404",
		},
		{
			name: "server returns invalid JSON",
			setupServer: func(t *testing.T) (string, func()) {
				socketPath := filepath.Join(t.TempDir(), "test-badjson.sock")
				listener, _ := net.Listen("unix", socketPath)
				ts := &http.Server{
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("{invalid json}"))
					}),
				}
				go ts.Serve(listener)
				return socketPath, func() { ts.Close(); listener.Close() }
			},
			wantError:  true,
			wantErrMsg: "failed to decode response",
		},
		{
			name: "server returns valid response",
			setupServer: func(t *testing.T) (string, func()) {
				socketPath := filepath.Join(t.TempDir(), "test-valid.sock")
				listener, _ := net.Listen("unix", socketPath)
				ts := &http.Server{
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						resp := wlapi.X509SVIDResponse{
							SPIFFEID:  "spiffe://example.org/workload",
							X509SVID:  "PEM certificate data",
							ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						json.NewEncoder(w).Encode(resp)
					}),
				}
				go ts.Serve(listener)
				return socketPath, func() { ts.Close(); listener.Close() }
			},
			wantError:  false,
			wantErrMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			socketPath, cleanup := tt.setupServer(t)
			defer cleanup()

			time.Sleep(50 * time.Millisecond)

			client := wlapi.NewClient(socketPath)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			resp, err := client.FetchX509SVID(ctx)

			if tt.wantError {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.GetSPIFFEID())
			}
		})
	}
}

// TestX509SVIDResponse_Methods tests response accessor methods
func TestX509SVIDResponse_Methods(t *testing.T) {
	t.Parallel()

	resp := &wlapi.X509SVIDResponse{
		SPIFFEID:  "spiffe://example.org/test",
		X509SVID:  "PEM data",
		ExpiresAt: 1234567890,
	}

	assert.Equal(t, "spiffe://example.org/test", resp.GetSPIFFEID())
	assert.Equal(t, "PEM data", resp.GetX509SVID())
	assert.Equal(t, int64(1234567890), resp.GetExpiresAt())
	assert.Equal(t, "spiffe://example.org/test", resp.ToIdentity())
}

// TestX509SVIDResponse_NilSafety tests nil response safety
func TestX509SVIDResponse_NilSafety(t *testing.T) {
	t.Parallel()

	var resp *wlapi.X509SVIDResponse

	assert.Empty(t, resp.GetSPIFFEID())
	assert.Empty(t, resp.GetX509SVID())
	assert.Equal(t, int64(0), resp.GetExpiresAt())
	assert.Empty(t, resp.ToIdentity())
}

// TestClient_ImplementsPort verifies Client implements WorkloadAPIClient interface
func TestClient_ImplementsPort(t *testing.T) {
	t.Parallel()

	client := wlapi.NewClient("/tmp/test.sock")
	var _ ports.WorkloadAPIClient = client
}

// TestX509SVIDResponse_ImplementsPort verifies response implements interface
func TestX509SVIDResponse_ImplementsPort(t *testing.T) {
	t.Parallel()

	resp := &wlapi.X509SVIDResponse{}
	var _ ports.X509SVIDResponse = resp
}

// TestClient_ContextTimeout tests context timeout handling
func TestClient_ContextTimeout(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "test-timeout.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	defer listener.Close()

	// Server that delays response
	ts := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}),
	}
	go ts.Serve(listener)
	defer ts.Close()

	time.Sleep(50 * time.Millisecond)

	client := wlapi.NewClient(socketPath)

	// Context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.FetchX509SVID(ctx)
	assert.Error(t, err)
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
			resp := wlapi.X509SVIDResponse{
				SPIFFEID:  "spiffe://example.org/workload",
				X509SVID:  "cert",
				ExpiresAt: time.Now().Unix(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}),
	}
	go ts.Serve(listener)
	defer ts.Close()

	time.Sleep(50 * time.Millisecond)

	client := wlapi.NewClient(socketPath)

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
