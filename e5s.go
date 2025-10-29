// Package e5s provides a simple API for building production-grade mTLS services
// with SPIFFE identity verification using SPIRE.
//
// This package wraps the lower-level pkg/identitytls and pkg/spire packages,
// providing a config-file-driven approach that requires minimal code.
//
// Quick Start:
//
// Server:
//
//	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	    peer, ok := e5s.PeerInfo(r)
//	    if !ok {
//	        http.Error(w, "unauthorized", http.StatusUnauthorized)
//	        return
//	    }
//	    fmt.Fprintf(w, "Hello %s\n", peer.ID.String())
//	})
//
//	shutdown, err := e5s.Start("e5s.yaml", handler)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer shutdown()
//
// Client:
//
//	client, shutdown, err := e5s.Client("e5s.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer shutdown()
//
//	resp, err := client.Get("https://secure-service:8443/api")
package e5s

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sufield/e5s/internal/config"
	"github.com/sufield/e5s/pkg/identitytls"
	"github.com/sufield/e5s/pkg/spire"
)

// Start starts a production-grade mTLS server using SPIRE.
//
// It loads configuration from the specified YAML file, connects to the SPIRE
// Workload API, configures mutual TLS with automatic certificate rotation,
// and starts serving the provided HTTP handler.
//
// The server enforces:
//   - TLS 1.3 minimum
//   - Mutual TLS (both server and client present certificates)
//   - SPIFFE ID verification of clients based on config policy
//   - Automatic certificate rotation (zero-downtime)
//
// Configuration (e5s.yaml):
//
//	spire:
//	  workload_socket: unix:///tmp/spire-agent/public/api.sock
//	server:
//	  listen_addr: ":8443"
//	  # Exactly one of these:
//	  allowed_client_spiffe_id: "spiffe://example.org/client"
//	  # OR
//	  allowed_client_trust_domain: "example.org"
//
// The handler can extract authenticated peer identity using PeerInfo(r) or PeerID(r).
//
// Returns:
//   - shutdown: function to gracefully stop the server and release resources
//   - error: if config loading, SPIRE connection, or server startup fails
//
// Shutdown semantics:
//   - The shutdown function is safe to call multiple times (idempotent)
//   - First call initiates graceful shutdown with 5-second timeout
//   - Subsequent calls return the result of the first shutdown
//   - Shutdown does NOT wait for in-flight requests (use sync.WaitGroup if needed)
//   - After shutdown, the SPIRE source is closed and cannot be reused
//
// Usage:
//
//	shutdown, err := e5s.Start("e5s.yaml", myHandler)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Wait for interrupt signal
//	sigChan := make(chan os.Signal, 1)
//	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
//	<-sigChan
//
//	// Gracefully shutdown
//	if err := shutdown(); err != nil {
//	    log.Printf("Shutdown error: %v", err)
//	    os.Exit(1)
//	}
//	os.Exit(0)
func Start(configPath string, handler http.Handler) (shutdown func() error, err error) {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Validate server configuration
	if err := config.ValidateServer(cfg); err != nil {
		return nil, fmt.Errorf("invalid server config: %w", err)
	}

	ctx := context.Background()

	// Connect to SPIRE Workload API
	source, err := spire.NewSource(ctx, spire.Config{
		WorkloadSocket: cfg.SPIRE.WorkloadSocket,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE source: %w", err)
	}

	// Get SDK X509Source for TLS config
	x509Source := source.X509Source()

	// Build server TLS config with client verification
	tlsCfg, err := identitytls.NewServerTLSConfig(
		ctx,
		x509Source,
		x509Source,
		identitytls.ServerConfig{
			AllowedClientID:          cfg.Server.AllowedClientSPIFFEID,
			AllowedClientTrustDomain: cfg.Server.AllowedClientTrustDomain,
		},
	)
	if err != nil {
		source.Close()
		return nil, fmt.Errorf("failed to create server TLS config: %w", err)
	}

	// Wrap handler to inject peer identity into request context
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if peer, ok := identitytls.ExtractPeerInfo(r); ok {
			r = r.WithContext(identitytls.WithPeerInfo(r.Context(), peer))
		}
		handler.ServeHTTP(w, r)
	})

	// Create HTTP server with mTLS
	srv := &http.Server{
		Addr:      cfg.Server.ListenAddr,
		Handler:   wrapped,
		TLSConfig: tlsCfg,
	}

	// Channel to capture server startup errors
	errCh := make(chan error, 1)

	// Start server in background
	go func() {
		// Empty cert/key paths - TLS config comes from tlsCfg.GetCertificate
		err := srv.ListenAndServeTLS("", "")
		// Only send non-graceful errors
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Give server a moment to bind or fail
	// This prevents returning success when bind fails immediately
	select {
	case err := <-errCh:
		source.Close()
		return nil, fmt.Errorf("server startup failed: %w", err)
	case <-time.After(100 * time.Millisecond):
		// Server started successfully (or will report error later via logs)
	}

	// Ensure shutdown is only executed once
	var shutdownOnce sync.Once
	var shutdownErr error

	// Return shutdown function
	shutdownFunc := func() error {
		shutdownOnce.Do(func() {
			// Create timeout context for graceful shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Stop accepting new connections and drain in-flight requests
			err1 := srv.Shutdown(ctx)

			// Release SPIRE resources
			err2 := source.Close()

			// Return first error encountered
			if err1 != nil {
				shutdownErr = err1
			} else {
				shutdownErr = err2
			}
		})
		return shutdownErr
	}

	return shutdownFunc, nil
}

// PeerInfo extracts the authenticated caller's full identity from a request.
//
// This function retrieves the peer identity that was verified during the mTLS
// handshake and stored in the request context by the e5s server middleware.
//
// IMPORTANT: This only works for requests served by a server started with e5s.Start().
// If called on a request from a different server (or before mTLS verification),
// it returns false.
//
// Returns:
//   - PeerInfo: complete authenticated identity (SPIFFE ID, trust domain, cert expiry)
//   - ok: true if identity was found, false otherwise
//
// Usage in handler:
//
//	func myHandler(w http.ResponseWriter, r *http.Request) {
//	    peer, ok := e5s.PeerInfo(r)
//	    if !ok {
//	        http.Error(w, "unauthorized", http.StatusUnauthorized)
//	        return
//	    }
//	    // Use peer for authorization decisions
//	    log.Printf("Request from %s (trust domain: %s, expires: %s)",
//	        peer.ID.String(), peer.ID.TrustDomain().Name(), peer.ExpiresAt)
//	}
func PeerInfo(r *http.Request) (identitytls.PeerInfo, bool) {
	return identitytls.PeerInfoFromContext(r.Context())
}

// PeerID extracts the authenticated caller's SPIFFE ID from a request.
//
// This is a convenience wrapper around PeerInfo() that returns only the SPIFFE ID.
// Use PeerInfo() if you need access to trust domain or certificate expiry.
//
// IMPORTANT: This only works for requests served by a server started with e5s.Start().
// If called on a request from a different server (or before mTLS verification),
// it returns false.
//
// Returns:
//   - spiffeID: the authenticated peer's SPIFFE ID (e.g., "spiffe://example.org/client")
//   - ok: true if identity was found, false otherwise
//
// Usage in handler:
//
//	func myHandler(w http.ResponseWriter, r *http.Request) {
//	    id, ok := e5s.PeerID(r)
//	    if !ok {
//	        http.Error(w, "unauthorized", http.StatusUnauthorized)
//	        return
//	    }
//	    // Use id for authorization decisions
//	    log.Printf("Request from %s", id)
//	}
func PeerID(r *http.Request) (string, bool) {
	peer, ok := identitytls.PeerInfoFromContext(r.Context())
	if !ok {
		return "", false
	}
	return peer.ID.String(), true
}

// Client returns an HTTP client configured for mTLS using SPIRE.
//
// The returned client will:
//   - Present the workload's SPIFFE ID via mTLS
//   - Verify the remote server's SPIFFE ID based on config policy
//   - Handle automatic certificate rotation
//   - Use TLS 1.3 minimum
//
// Configuration (e5s.yaml):
//
//	spire:
//	  workload_socket: unix:///tmp/spire-agent/public/api.sock
//	client:
//	  # Exactly one of these:
//	  expected_server_spiffe_id: "spiffe://example.org/api-server"
//	  # OR
//	  expected_server_trust_domain: "example.org"
//
// Returns:
//   - client: HTTP client ready for mTLS connections
//   - shutdown: function to release SPIRE resources (should be deferred)
//   - error: if config loading, SPIRE connection, or TLS setup fails
//
// Shutdown semantics:
//   - The shutdown function is safe to call multiple times (idempotent)
//   - First call closes the SPIRE source
//   - Subsequent calls return the result of the first close
//   - After shutdown, the client can still complete in-flight requests
//   - New requests after shutdown will fail with certificate errors
//
// Usage:
//
//	client, shutdown, err := e5s.Client("e5s.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer shutdown()
//
//	resp, err := client.Get("https://secure-service:8443/api")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer resp.Body.Close()
func Client(configPath string) (*http.Client, func() error, error) {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Validate client configuration
	if err := config.ValidateClient(cfg); err != nil {
		return nil, nil, fmt.Errorf("invalid client config: %w", err)
	}

	ctx := context.Background()

	// Connect to SPIRE Workload API
	source, err := spire.NewSource(ctx, spire.Config{
		WorkloadSocket: cfg.SPIRE.WorkloadSocket,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create SPIRE source: %w", err)
	}

	// Get SDK X509Source for TLS config
	x509Source := source.X509Source()

	// Build client TLS config with server verification
	tlsCfg, err := identitytls.NewClientTLSConfig(
		ctx,
		x509Source,
		x509Source,
		identitytls.ClientConfig{
			ExpectedServerID:          cfg.Client.ExpectedServerSPIFFEID,
			ExpectedServerTrustDomain: cfg.Client.ExpectedServerTrustDomain,
		},
	)
	if err != nil {
		source.Close()
		return nil, nil, fmt.Errorf("failed to create client TLS config: %w", err)
	}

	// Create HTTP client with mTLS
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}

	// Ensure shutdown is only executed once
	var shutdownOnce sync.Once
	var shutdownErr error

	// Return client and shutdown function
	shutdownFunc := func() error {
		shutdownOnce.Do(func() {
			shutdownErr = source.Close()
		})
		return shutdownErr
	}

	return httpClient, shutdownFunc, nil
}
