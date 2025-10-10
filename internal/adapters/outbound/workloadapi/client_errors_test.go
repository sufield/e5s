package workloadapi_test

// Workload API Client Error Handling Tests
//
// These tests verify client behavior with various error conditions including
// server errors, invalid responses, socket errors, and timeouts.
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/workloadapi/... -v -run TestClient.*Error
//	go test ./internal/adapters/outbound/workloadapi/... -v -run TestClient_FetchX509SVID_TableDriven
//	go test ./internal/adapters/outbound/workloadapi/... -cover

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	wlapi "github.com/pocket/hexagon/spire/internal/adapters/outbound/workloadapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// TestClient_FetchX509SVID_TableDriven tests various scenarios
func TestClient_FetchX509SVID_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupServer func(t *testing.T) (string, func())
		wantError   bool
		wantErrMsg  string
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
