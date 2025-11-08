//go:build integration
// +build integration

package e5s_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
