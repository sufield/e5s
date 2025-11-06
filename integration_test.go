//go:build integration
// +build integration

package e5s_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/sufield/e5s"
)

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
	const cfgPath = "testdata/e5s.integration.yaml"

	// Verify config file exists
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("missing integration config %s: %v", cfgPath, err)
	}

	// Handler inspects the authenticated SPIFFE ID
	var seenID string
	var seenPeerInfo e5s.Peer

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
// This test verifies that Serve() can start a server using environment
// variable configuration and handle graceful shutdown.
func TestE2E_Serve(t *testing.T) {
	// This test is optional since Serve() is just a wrapper around Start()
	// Skip for now to keep integration tests focused
	t.Skip("Serve() is tested indirectly via Start()")
}

// TestE2E_Get tests the Get convenience function.
//
// This test verifies that Get() can make requests using configuration
// from environment variables.
func TestE2E_Get(t *testing.T) {
	const cfgPath = "testdata/e5s.integration.yaml"

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
