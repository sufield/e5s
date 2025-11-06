//go:build integration
// +build integration

package spiffehttp_test

import (
	"context"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/sufield/e5s/internal/testhelpers"
	"github.com/sufield/e5s/spiffehttp"
	"github.com/sufield/e5s/spire"
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

// TestIntegration_ServerClientHandshake tests that NewServerTLSConfig and
// NewClientTLSConfig can successfully handshake using real SPIRE sources.
//
// This verifies:
// - Real SPIRE identity source can be used for TLS config
// - Server and client can perform mTLS handshake
// - PeerFromRequest extracts peer identity correctly
// - Trust domain-based authorization works
func TestIntegration_ServerClientHandshake(t *testing.T) {
	// Get or setup SPIRE infrastructure
	workloadSocket := getOrSetupSPIRE(t)
	const trustDomain = "example.org"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create SPIRE identity source
	t.Logf("Connecting to SPIRE at: %s", workloadSocket)
	src, err := spire.NewIdentitySource(ctx, spire.Config{
		WorkloadSocket:      workloadSocket,
		InitialFetchTimeout: 20 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create identity source: %v", err)
	}
	defer src.Close()

	x509src := src.X509Source()
	if x509src == nil {
		t.Fatal("X509Source returned nil")
	}

	// Create server TLS config
	serverTLS, err := spiffehttp.NewServerTLSConfig(ctx, x509src, x509src, spiffehttp.ServerConfig{
		AllowedClientTrustDomain: trustDomain,
	})
	if err != nil {
		t.Fatalf("NewServerTLSConfig failed: %v", err)
	}
	t.Log("✓ Created server TLS config")

	// Create client TLS config
	clientTLS, err := spiffehttp.NewClientTLSConfig(ctx, x509src, x509src, spiffehttp.ClientConfig{
		ExpectedServerTrustDomain: trustDomain,
	})
	if err != nil {
		t.Fatalf("NewClientTLSConfig failed: %v", err)
	}
	t.Log("✓ Created client TLS config")

	// Track peer info extracted in handler
	var seenPeer spiffehttp.Peer

	// Create handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, ok := spiffehttp.PeerFromRequest(r)
		if !ok {
			http.Error(w, "unauthorized: no peer identity", http.StatusUnauthorized)
			return
		}
		seenPeer = peer
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("authenticated"))
	})

	// Create a real HTTP server with our TLS config (not httptest which generates its own certs)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	server := &http.Server{
		Handler:   handler,
		TLSConfig: serverTLS,
	}

	// Start server in background
	go func() {
		if err := server.ServeTLS(listener, "", ""); err != nil && err != http.ErrServerClosed {
			t.Logf("server error: %v", err)
		}
	}()
	defer server.Close()

	serverURL := "https://" + listener.Addr().String()
	t.Logf("Test server listening at: %s", serverURL)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create client with mTLS config
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: clientTLS,
		},
		Timeout: 10 * time.Second,
	}

	// Make request
	resp, err := client.Get(serverURL)
	if err != nil {
		t.Fatalf("client request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	t.Log("✓ mTLS handshake succeeded")

	// Verify peer was extracted
	if seenPeer.ID.String() == "" {
		t.Fatal("expected handler to see peer SPIFFE ID")
	}
	if seenPeer.ID.TrustDomain().Name() != trustDomain {
		t.Errorf("peer trust domain = %s, want %s", seenPeer.ID.TrustDomain().Name(), trustDomain)
	}
	t.Logf("✓ Verified peer identity: %s", seenPeer.ID)
}

// TestIntegration_SpecificIDAuthorization tests that exact SPIFFE ID matching
// works correctly with real SPIRE.
func TestIntegration_SpecificIDAuthorization(t *testing.T) {
	workloadSocket := getOrSetupSPIRE(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create identity source
	src, err := spire.NewIdentitySource(ctx, spire.Config{
		WorkloadSocket:      workloadSocket,
		InitialFetchTimeout: 20 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create identity source: %v", err)
	}
	defer src.Close()

	x509src := src.X509Source()

	// Get our own SPIFFE ID
	svid, err := x509src.GetX509SVID()
	if err != nil {
		t.Fatalf("failed to get SVID: %v", err)
	}
	myID := svid.ID.String()
	t.Logf("Our SPIFFE ID: %s", myID)

	// Server allows only our specific ID
	serverTLS, err := spiffehttp.NewServerTLSConfig(ctx, x509src, x509src, spiffehttp.ServerConfig{
		AllowedClientID: myID,
	})
	if err != nil {
		t.Fatalf("NewServerTLSConfig failed: %v", err)
	}

	// Client expects server with our ID (since we're using same identity)
	clientTLS, err := spiffehttp.NewClientTLSConfig(ctx, x509src, x509src, spiffehttp.ClientConfig{
		ExpectedServerID: myID,
	})
	if err != nil {
		t.Fatalf("NewClientTLSConfig failed: %v", err)
	}

	// Create server with real TLS config
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		TLSConfig: serverTLS,
	}

	go func() {
		if err := server.ServeTLS(listener, "", ""); err != nil && err != http.ErrServerClosed {
			t.Logf("server error: %v", err)
		}
	}()
	defer server.Close()

	serverURL := "https://" + listener.Addr().String()
	time.Sleep(100 * time.Millisecond)

	// Make request - should succeed because IDs match
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: clientTLS,
		},
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(serverURL)
	if err != nil {
		t.Fatalf("request failed (IDs should match): %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	t.Log("✓ Specific SPIFFE ID authorization works")
}

// TestIntegration_PeerContext tests WithPeer and PeerFromContext.
func TestIntegration_PeerContext(t *testing.T) {
	workloadSocket := getOrSetupSPIRE(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	src, err := spire.NewIdentitySource(ctx, spire.Config{
		WorkloadSocket:      workloadSocket,
		InitialFetchTimeout: 20 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create identity source: %v", err)
	}
	defer src.Close()

	x509src := src.X509Source()

	serverTLS, err := spiffehttp.NewServerTLSConfig(ctx, x509src, x509src, spiffehttp.ServerConfig{
		AllowedClientTrustDomain: "example.org",
	})
	if err != nil {
		t.Fatalf("NewServerTLSConfig failed: %v", err)
	}

	clientTLS, err := spiffehttp.NewClientTLSConfig(ctx, x509src, x509src, spiffehttp.ClientConfig{
		ExpectedServerTrustDomain: "example.org",
	})
	if err != nil {
		t.Fatalf("NewClientTLSConfig failed: %v", err)
	}

	// Middleware that uses WithPeer
	var contextPeer spiffehttp.Peer
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer, ok := spiffehttp.PeerFromRequest(r)
			if !ok {
				http.Error(w, "no peer", http.StatusUnauthorized)
				return
			}
			// Attach to context
			ctx := spiffehttp.WithPeer(r.Context(), peer)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Handler that uses PeerFromContext
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, ok := spiffehttp.PeerFromContext(r.Context())
		if !ok {
			http.Error(w, "peer not in context", http.StatusInternalServerError)
			return
		}
		contextPeer = peer
		w.WriteHeader(http.StatusOK)
	})

	// Create server with real TLS config
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	server := &http.Server{
		Handler:   middleware(handler),
		TLSConfig: serverTLS,
	}

	go func() {
		if err := server.ServeTLS(listener, "", ""); err != nil && err != http.ErrServerClosed {
			t.Logf("server error: %v", err)
		}
	}()
	defer server.Close()

	serverURL := "https://" + listener.Addr().String()
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: clientTLS,
		},
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(serverURL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	if contextPeer.ID.String() == "" {
		t.Fatal("expected handler to extract peer from context")
	}

	t.Logf("✓ Context peer flow works: %s", contextPeer.ID)
}
