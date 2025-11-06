//go:build integration
// +build integration

package e5s_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sufield/e5s"
	"github.com/sufield/e5s/internal/testhelpers"
	"github.com/sufield/e5s/spiffehttp"
)

// getOrSetupSPIRE returns a socket path to use for testing.
// If SPIFFE_ENDPOINT_SOCKET is set, it uses the existing SPIRE agent.
// Otherwise, it starts a new SPIRE server and agent for the test.
func getOrSetupSPIRE(t *testing.T) string {
	socketPath := os.Getenv("SPIFFE_ENDPOINT_SOCKET")
	if socketPath != "" {
		t.Logf("Using existing SPIRE agent from environment: %s", socketPath)
		return socketPath
	}

	// No existing SPIRE, start our own
	st := testhelpers.SetupSPIRE(t)
	return "unix://" + st.SocketPath
}

// TestE2E_Start_Client_PeerID tests the high-level e5s API end-to-end.
//
// This test verifies:
// - Starting an mTLS server with Start()
// - Creating an mTLS client with Client()
// - Successful TLS handshake using real SPIRE SVIDs
// - Extracting peer SPIFFE ID with PeerID()
//
// Requires: SPIRE server and agent running with workload registration
func TestE2E_Start_Client_PeerID(t *testing.T) {
	// Get or setup SPIRE infrastructure
	socketPath := getOrSetupSPIRE(t)

	// Create temporary config file with dynamic socket path
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "e5s-test.yaml")

	configContent := fmt.Sprintf(`spire:
  workload_socket: "%s"
  initial_fetch_timeout: "30s"

server:
  listen_addr: ":18443"
  allowed_client_trust_domain: "example.org"

client:
  server_url: "https://localhost:18443/api"
  expected_server_trust_domain: "example.org"
`, socketPath)

	if err := os.WriteFile(cfgPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Handler inspects the authenticated SPIFFE ID
	var seenID string
	var seenPeerInfo spiffehttp.Peer

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Test PeerID extraction
		id, ok := e5s.PeerID(r)
		if !ok {
			http.Error(w, "unauthorized: no SPIFFE ID", http.StatusUnauthorized)
			return
		}
		seenID = id

		// Test PeerInfo extraction
		peer, ok := e5s.PeerInfo(r)
		if !ok {
			http.Error(w, "unauthorized: no peer info", http.StatusUnauthorized)
			return
		}
		seenPeerInfo = peer

		fmt.Fprintf(w, "hello %s", id)
	})

	// Start server with integration config
	t.Logf("Starting server with config: %s", cfgPath)
	shutdown, err := e5s.Start(cfgPath, handler)
	if err != nil {
		t.Fatalf("failed to start e5s server: %v", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			t.Errorf("shutdown failed: %v", err)
		}
	}()

	// Give the server time to bind to port
	time.Sleep(500 * time.Millisecond)

	// Create client with same config
	t.Logf("Creating client with config: %s", cfgPath)
	client, cleanup, err := e5s.Client(cfgPath)
	if err != nil {
		t.Fatalf("failed to create e5s client: %v", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			t.Errorf("client cleanup failed: %v", err)
		}
	}()

	// Make request to server
	resp, err := client.Get("https://localhost:18443/api")
	if err != nil {
		t.Fatalf("client request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d body=%q", resp.StatusCode, string(body))
	}

	// Verify handler saw authenticated peer
	if seenID == "" {
		t.Fatal("expected handler to see authenticated SPIFFE ID")
	}
	t.Logf("✓ Verified peer SPIFFE ID: %s", seenID)

	// Verify PeerInfo fields
	if seenPeerInfo.ID.String() == "" {
		t.Fatal("expected peer info to contain SPIFFE ID")
	}
	if seenPeerInfo.ID.String() != seenID {
		t.Errorf("PeerInfo.ID = %s, want %s", seenPeerInfo.ID, seenID)
	}
	if seenPeerInfo.ExpiresAt.IsZero() {
		t.Error("expected peer info to contain certificate expiration time")
	}
	if seenPeerInfo.ExpiresAt.Before(time.Now()) {
		t.Error("peer certificate already expired")
	}
	t.Logf("✓ Verified peer info: trust domain=%s, expires=%s",
		seenPeerInfo.ID.TrustDomain().Name(), seenPeerInfo.ExpiresAt)
}

// TestE2E_Serve tests the Serve convenience function.
//
// This test verifies that Serve() correctly:
//  1. Reads configuration from E5S_CONFIG environment variable
//  2. Starts the server and responds to requests
//  3. Uses Start() internally (shutdown path tested via Start)
func TestE2E_Serve(t *testing.T) {
	// Get or setup SPIRE infrastructure
	socketPath := getOrSetupSPIRE(t)

	// Create temporary config file
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "e5s-test.yaml")

	configContent := fmt.Sprintf(`spire:
  workload_socket: "%s"
  initial_fetch_timeout: "30s"

server:
  listen_addr: ":18444"
  allowed_client_trust_domain: "example.org"

client:
  server_url: "https://localhost:18444/api"
  expected_server_trust_domain: "example.org"
`, socketPath)

	if err := os.WriteFile(cfgPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Set E5S_CONFIG environment variable
	t.Setenv("E5S_CONFIG", cfgPath)

	var requestReceived bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		fmt.Fprint(w, "ok")
	})

	// Run Serve in a goroutine since it blocks until signal
	serverErr := make(chan error, 1)

	go func() {
		serverErr <- e5s.Serve(handler)
	}()

	// Give server time to start
	time.Sleep(3 * time.Second)

	// Create client
	client, cleanup, err := e5s.Client(cfgPath)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer cleanup()

	// Poll server until it's ready (with timeout)
	var resp *http.Response
	var lastErr error
	maxRetries := 20
	for i := 0; i < maxRetries; i++ {
		resp, lastErr = client.Get("https://localhost:18444/api")
		if lastErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Check if we successfully connected
	if lastErr != nil {
		// Check if server goroutine errored
		select {
		case err := <-serverErr:
			t.Fatalf("Server failed: %v", err)
		default:
			t.Fatalf("client request failed after %d retries: %v", maxRetries, lastErr)
		}
	}
	defer resp.Body.Close()

	t.Log("✓ Server started via Serve()")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	if !requestReceived {
		t.Error("expected handler to receive request")
	}

	t.Log("✓ Server is responding to requests")
	t.Log("✓ Serve() uses E5S_CONFIG correctly")

	// Send SIGINT to trigger graceful shutdown
	// Note: This sends signal to the entire process, which will cancel the
	// signal.NotifyContext in Serve(). This is safe in tests since we're
	// only running one Serve() at a time.
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("failed to find process: %v", err)
	}

	if err := proc.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("failed to send SIGINT: %v", err)
	}

	t.Log("✓ Sent SIGINT for graceful shutdown")

	// Wait for Serve to return
	select {
	case err := <-serverErr:
		if err != nil {
			t.Errorf("Serve returned error: %v", err)
		}
		t.Log("✓ Server shut down gracefully on signal")
	case <-time.After(10 * time.Second):
		t.Fatal("Server did not shut down within timeout")
	}
}

// TestE2E_Serve_MissingEnvVar tests that Serve returns an error when E5S_CONFIG is not set.
func TestE2E_Serve_MissingEnvVar(t *testing.T) {
	// Ensure E5S_CONFIG is not set
	os.Unsetenv("E5S_CONFIG")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})

	err := e5s.Serve(handler)
	if err == nil {
		t.Fatal("expected error when E5S_CONFIG not set, got nil")
	}

	if !strings.Contains(err.Error(), "E5S_CONFIG") {
		t.Errorf("expected error to mention E5S_CONFIG, got: %v", err)
	}

	t.Logf("✓ Serve correctly fails when E5S_CONFIG not set: %v", err)
}

// TestE2E_Get tests the Get convenience function.
//
// This test verifies that Get() can make requests using configuration
// from environment variables.
func TestE2E_Get(t *testing.T) {
	// Get or setup SPIRE infrastructure
	socketPath := getOrSetupSPIRE(t)

	// Create temporary config file
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "e5s-test.yaml")

	configContent := fmt.Sprintf(`spire:
  workload_socket: "%s"
  initial_fetch_timeout: "30s"

server:
  listen_addr: ":18443"
  allowed_client_trust_domain: "example.org"

client:
  server_url: "https://localhost:18443/api"
  expected_server_trust_domain: "example.org"
`, socketPath)

	if err := os.WriteFile(cfgPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Set environment variable for Get to use
	t.Setenv("E5S_CONFIG", cfgPath)

	// Start server first
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})

	shutdown, err := e5s.Start(cfgPath, handler)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer shutdown()

	time.Sleep(500 * time.Millisecond)

	// Use Get() convenience function
	resp, err := e5s.Get("https://localhost:18443/api")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Get status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("Get body = %q, want %q", body, "ok")
	}

	t.Log("✓ Verified Get() convenience function")
}
