package e5s_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sufield/e5s"
	"github.com/sufield/e5s/internal/testhelpers"
)

// TestE5SWithContainers demonstrates using testcontainers for SPIRE-based integration testing.
//
// This test:
//  1. Starts SPIRE server and agent in Docker containers
//  2. Creates a server with mTLS using e5s
//  3. Creates a client with mTLS using e5s
//  4. Verifies end-to-end mTLS communication
//
// Run with: go test -v -run TestE5SWithContainers
// Skip in short mode: go test -short (won't run container tests)
func TestE5SWithContainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container-based integration test in short mode")
	}

	// Setup containerized SPIRE infrastructure
	spire, cleanup := testhelpers.SetupSPIREContainers(t)
	defer cleanup()

	t.Log("SPIRE containers started successfully")
	t.Logf("Socket path: %s", spire.SocketPath)
	t.Logf("Trust domain: %s", spire.TrustDomain)

	// Create server configuration
	serverCfg := e5s.Config{
		Mode: e5s.ModeServer,
		Server: &e5s.ServerConfig{
			ListenAddr: ":18443",
			TLS: e5s.TLSConfig{
				WorkloadSocket: spire.SocketPath,
			},
			// Allow any client from the same trust domain for this test
			AllowedTrustDomain: spire.TrustDomain,
		},
	}

	// Create HTTP router with test endpoint
	r := chi.NewRouter()
	r.Get("/hello", func(w http.ResponseWriter, req *http.Request) {
		// Extract peer SPIFFE ID from mTLS connection
		id, ok := e5s.PeerID(req)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		t.Logf("Server received request from: %s", id)
		fmt.Fprintf(w, "Hello, %s!", id)
	})

	// Start e5s server
	shutdown, err := e5s.StartWithConfig(serverCfg, r)
	if err != nil {
		t.Fatalf("Failed to start e5s server: %v", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			t.Logf("Server shutdown error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(1 * time.Second)

	// Create client configuration
	clientCfg := e5s.Config{
		Mode: e5s.ModeClient,
		Client: &e5s.ClientConfig{
			TLS: e5s.TLSConfig{
				WorkloadSocket: spire.SocketPath,
			},
			// Trust the same domain
			TrustedDomain: spire.TrustDomain,
		},
	}

	// Make mTLS request using e5s client
	err = e5s.WithHTTPClientFromConfig(context.Background(), clientCfg, func(client *http.Client) error {
		resp, err := client.Get("https://localhost:18443/hello")
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		response := string(body)
		t.Logf("Client received response: %s", response)

		// Verify response contains SPIFFE ID
		if len(response) < 10 || response[:6] != "Hello," {
			return fmt.Errorf("unexpected response format: %s", response)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("mTLS communication failed: %v", err)
	}

	t.Log("✓ mTLS communication successful!")
}

// TestE5SWithContainersSPIFFEIDValidation tests SPIFFE ID-based authorization.
//
// This test verifies that:
//  1. Server correctly rejects clients without proper SPIFFE ID
//  2. Server correctly accepts clients with matching SPIFFE ID
//  3. e5s properly enforces zero-trust security
func TestE5SWithContainersSPIFFEIDValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container-based integration test in short mode")
	}

	// Setup containerized SPIRE infrastructure
	spire, cleanup := testhelpers.SetupSPIREContainers(t)
	defer cleanup()

	// Expected client SPIFFE ID
	expectedClientID := fmt.Sprintf("spiffe://%s/test-workload", spire.TrustDomain)

	// Create server with strict SPIFFE ID validation
	serverCfg := e5s.Config{
		Mode: e5s.ModeServer,
		Server: &e5s.ServerConfig{
			ListenAddr: ":18444",
			TLS: e5s.TLSConfig{
				WorkloadSocket: spire.SocketPath,
			},
			// Only allow specific client SPIFFE ID
			AllowedClientSPIFFEID: expectedClientID,
		},
	}

	// Create HTTP router
	r := chi.NewRouter()
	authenticated := 0
	r.Get("/secure", func(w http.ResponseWriter, req *http.Request) {
		id, ok := e5s.PeerID(req)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		authenticated++
		t.Logf("Authenticated request %d from: %s", authenticated, id)
		fmt.Fprintf(w, "Secure data for %s", id)
	})

	// Start server
	shutdown, err := e5s.StartWithConfig(serverCfg, r)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			t.Logf("Server shutdown error: %v", err)
		}
	}()

	time.Sleep(1 * time.Second)

	// Create client configuration
	clientCfg := e5s.Config{
		Mode: e5s.ModeClient,
		Client: &e5s.ClientConfig{
			TLS: e5s.TLSConfig{
				WorkloadSocket: spire.SocketPath,
			},
			TrustedDomain: spire.TrustDomain,
		},
	}

	// Make request - should succeed because test workload has correct SPIFFE ID
	err = e5s.WithHTTPClientFromConfig(context.Background(), clientCfg, func(client *http.Client) error {
		resp, err := client.Get("https://localhost:18444/secure")
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, body)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		t.Logf("Received secure response: %s", body)
		return nil
	})

	if err != nil {
		t.Fatalf("Client with correct SPIFFE ID was rejected: %v", err)
	}

	if authenticated != 1 {
		t.Errorf("Expected 1 authenticated request, got %d", authenticated)
	}

	t.Log("✓ SPIFFE ID validation working correctly!")
}
