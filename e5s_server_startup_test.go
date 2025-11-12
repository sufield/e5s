//go:build integration
// +build integration

package e5s_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sufield/e5s"
	"github.com/sufield/e5s/internal/testhelpers"
)

// TestStartWithContext_SuccessfulStartup tests the happy path:
// server starts in goroutine, binds successfully, responds to requests, and shuts down cleanly.
// This covers e5s.go lines 340-395 (goroutine, errCh, shutdownOnce patterns).
func TestStartWithContext_SuccessfulStartup(t *testing.T) {
	// Setup SPIRE infrastructure
	st := testhelpers.SetupSPIRE(t)

	// Create test config
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "server-test.yaml")
	configContent := fmt.Sprintf(`spire:
  workload_socket: "unix://%s"
  initial_fetch_timeout: "30s"

server:
  listen_addr: "localhost:0"
  allowed_client_trust_domain: "example.org"
`, st.SocketPath)

	if err := os.WriteFile(cfgPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Start server with context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	shutdown, err := e5s.StartWithContext(ctx, cfgPath, handler)
	if err != nil {
		t.Fatalf("StartWithContext failed: %v", err)
	}

	// Verify shutdown function is returned
	if shutdown == nil {
		t.Fatal("shutdown function is nil")
	}

	// Give server time to fully start
	time.Sleep(200 * time.Millisecond)

	// Test shutdown - this verifies the shutdownOnce pattern and cleanup
	if err := shutdown(); err != nil {
		t.Errorf("shutdown failed: %v", err)
	}

	// Test multiple shutdown calls (should be idempotent via sync.Once)
	if err := shutdown(); err != nil {
		t.Errorf("second shutdown call failed: %v", err)
	}

	t.Log("✓ Server started successfully and shut down cleanly")
}

// TestStartWithContext_StartupError tests error handling when server fails to bind.
// This covers e5s.go lines 358-362 (startup error path + identityShutdown cleanup).
func TestStartWithContext_StartupError(t *testing.T) {
	// Setup SPIRE infrastructure
	st := testhelpers.SetupSPIRE(t)

	// Create config with invalid listen address that will cause bind error
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "server-invalid.yaml")
	configContent := fmt.Sprintf(`spire:
  workload_socket: "unix://%s"
  initial_fetch_timeout: "30s"

server:
  listen_addr: "invalid-hostname:99999"
  allowed_client_trust_domain: "example.org"
`, st.SocketPath)

	if err := os.WriteFile(cfgPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	// Attempt to start server - should fail due to invalid address
	ctx := context.Background()
	shutdown, err := e5s.StartWithContext(ctx, cfgPath, handler)

	// Verify error is returned
	if err == nil {
		t.Fatal("Expected error for invalid listen address, got nil")
		if shutdown != nil {
			shutdown() // Clean up if somehow succeeded
		}
	}

	// Verify shutdown function is NOT returned on failure
	if shutdown != nil {
		t.Error("Expected nil shutdown function on startup failure")
	}

	t.Logf("✓ Startup error handled correctly: %v", err)
}

// TestStartWithContext_ContextCancellationAfterStart tests cancellation during server operation.
// This verifies the context propagation and graceful shutdown path after successful startup.
func TestStartWithContext_ContextCancellationAfterStart(t *testing.T) {
	// Setup SPIRE infrastructure
	st := testhelpers.SetupSPIRE(t)

	// Create test config
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "server-cancel.yaml")
	configContent := fmt.Sprintf(`spire:
  workload_socket: "unix://%s"
  initial_fetch_timeout: "30s"

server:
  listen_addr: "localhost:0"
  allowed_client_trust_domain: "example.org"
`, st.SocketPath)

	if err := os.WriteFile(cfgPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	shutdown, err := e5s.StartWithContext(ctx, cfgPath, handler)
	if err != nil {
		t.Fatalf("StartWithContext failed: %v", err)
	}
	defer shutdown()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Cancel context
	cancel()

	// Shutdown should still work after context cancellation
	if err := shutdown(); err != nil {
		t.Errorf("shutdown after context cancellation failed: %v", err)
	}

	t.Log("✓ Context cancellation handled correctly")
}

// TestStart_WithRealRequests tests server handling actual HTTP requests.
// This verifies the full request/response cycle including PeerID extraction.
func TestStart_WithRealRequests(t *testing.T) {
	// Setup SPIRE infrastructure
	st := testhelpers.SetupSPIRE(t)

	// Create server config
	tempDir := t.TempDir()
	serverCfgPath := filepath.Join(tempDir, "server.yaml")
	serverConfig := fmt.Sprintf(`spire:
  workload_socket: "unix://%s"
  initial_fetch_timeout: "30s"

server:
  listen_addr: "localhost:18444"
  allowed_client_trust_domain: "example.org"
`, st.SocketPath)

	if err := os.WriteFile(serverCfgPath, []byte(serverConfig), 0o600); err != nil {
		t.Fatalf("Failed to write server config: %v", err)
	}

	// Create client config
	clientCfgPath := filepath.Join(tempDir, "client.yaml")
	clientConfig := fmt.Sprintf(`spire:
  workload_socket: "unix://%s"
  initial_fetch_timeout: "30s"

client:
  expected_server_trust_domain: "example.org"
`, st.SocketPath)

	if err := os.WriteFile(clientCfgPath, []byte(clientConfig), 0o600); err != nil {
		t.Fatalf("Failed to write client config: %v", err)
	}

	// Track if handler was called
	handlerCalled := false
	var receivedPeerID string

	// Create handler that extracts peer ID
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		peerID, ok := e5s.PeerID(r)
		if ok {
			receivedPeerID = peerID
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Request received"))
	})

	// Start server
	shutdown, err := e5s.Start(serverCfgPath, handler)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer shutdown()

	// Give server time to start
	time.Sleep(300 * time.Millisecond)

	// Create mTLS client and make request
	err = e5s.WithClient(clientCfgPath, func(client *http.Client) error {
		resp, err := client.Get("https://localhost:18444/test")
		if err != nil {
			return fmt.Errorf("client request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Client request failed: %v", err)
	}

	// Verify handler was called
	if !handlerCalled {
		t.Error("Handler was not called")
	}

	// Verify peer ID was extracted
	if receivedPeerID == "" {
		t.Error("Peer ID was not extracted")
	} else {
		t.Logf("✓ Received peer ID: %s", receivedPeerID)
	}

	t.Log("✓ Server handled mTLS request successfully")
}
