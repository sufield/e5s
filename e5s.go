// Package e5s provides a simple API for building production-grade mTLS services
// with SPIFFE identity verification using SPIRE.
//
// This package wraps the lower-level spiffehttp and spire packages,
// providing a config-file-driven approach that requires minimal code.
//
// See docs/ARCHITECTURE.md for layering details.
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

	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/sufield/e5s/internal/config"
	"github.com/sufield/e5s/spiffehttp"
	"github.com/sufield/e5s/spire"
)

// newSPIRESource initializes the SPIRE identity source and returns:
//   - x509Source: the X.509 source used for TLS
//   - shutdown: an idempotent function that closes the source
func newSPIRESource(
	ctx context.Context,
	workloadSocket string,
	initialFetchTimeout time.Duration,
) (x509Source *workloadapi.X509Source, shutdown func() error, err error) {
	src, err := spire.NewIdentitySource(ctx, spire.Config{
		WorkloadSocket:      workloadSocket,
		InitialFetchTimeout: initialFetchTimeout,
	})
	if err != nil {
		return nil, nil, err
	}

	x509 := src.X509Source()

	var once sync.Once
	var shutdownErr error
	shutdown = func() error {
		once.Do(func() {
			shutdownErr = src.Close()
		})
		return shutdownErr
	}

	return x509, shutdown, nil
}

// buildServer constructs the HTTP server and SPIRE identity source used by both Start and StartSingleThread.
//
// This internal helper factors out common setup logic to ensure both execution modes use
// identical configuration, SPIRE wiring, and TLS setup.
//
// Returns:
//   - srv: configured HTTP server ready to serve
//   - identityShutdown: function to release SPIRE resources (idempotent)
//   - err: if config loading, SPIRE connection, or TLS setup fails
func buildServer(configPath string, handler http.Handler) (
	srv *http.Server,
	identityShutdown func() error,
	err error,
) {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Validate server configuration and get parsed authorization policy
	spireConfig, authz, err := config.ValidateServer(&cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid server config: %w", err)
	}

	ctx := context.Background()

	// Centralized SPIRE setup
	x509Source, identityShutdown, err := newSPIRESource(
		ctx,
		cfg.SPIRE.WorkloadSocket,
		spireConfig.InitialFetchTimeout,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create SPIRE source: %w", err)
	}

	// Build server TLS config with client verification
	// Note: cfg.* fields are already validated in config.ValidateServer.
	// spiffehttp still performs its own validation because it is a lower-level API.
	tlsCfg, err := spiffehttp.NewServerTLSConfig(
		ctx,
		x509Source,
		x509Source,
		spiffehttp.ServerConfig{
			AllowedClientID:          cfg.Server.AllowedClientSPIFFEID,
			AllowedClientTrustDomain: cfg.Server.AllowedClientTrustDomain,
		},
	)
	// Silence unused variable warning
	_ = authz
	if err != nil {
		if shutdownErr := identityShutdown(); shutdownErr != nil {
			return nil, nil, fmt.Errorf("failed to create server TLS config: %w (cleanup error: %v)", err, shutdownErr)
		}
		return nil, nil, fmt.Errorf("failed to create server TLS config: %w", err)
	}

	// Wrap handler to inject peer identity into request context
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if peer, ok := spiffehttp.PeerFromRequest(r); ok {
			r = r.WithContext(spiffehttp.WithPeer(r.Context(), peer))
		}
		handler.ServeHTTP(w, r)
	})

	// Create HTTP server with mTLS
	srv = &http.Server{
		Addr:              cfg.Server.ListenAddr,
		Handler:           wrapped,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
	}

	return srv, identityShutdown, nil
}

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
	srv, identityShutdown, err := buildServer(configPath, handler)
	if err != nil {
		return nil, err
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
		if shutdownErr := identityShutdown(); shutdownErr != nil {
			return nil, fmt.Errorf("server startup failed: %w (cleanup error: %v)", err, shutdownErr)
		}
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
			err2 := identityShutdown()

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

// StartSingleThread starts an mTLS server using SPIRE and blocks in the calling goroutine.
//
// This function provides a debug-friendly execution mode that uses the same configuration,
// SPIRE integration, and TLS setup as Start(), but runs the HTTP server synchronously in
// the current goroutine instead of spawning a background goroutine.
//
// go-spiffe behavior (including automatic certificate rotation) is unchanged; this mode
// only removes the goroutine that e5s itself creates for the HTTP server.
//
// This is useful for:
//   - Debugging with IDE step-through (predictable call stack)
//   - Isolating concurrency issues (is it e5s threading or something else?)
//   - Simplified testing scenarios
//   - Learning the library internals
//
// Configuration is identical to Start() - same e5s.yaml format:
//
//	spire:
//	  workload_socket: unix:///tmp/spire-agent/public/api.sock
//	server:
//	  listen_addr: ":8443"
//	  allowed_client_trust_domain: "example.org"
//
// Differences from Start():
//   - BLOCKS in the current goroutine (does not return until server stops)
//   - Does NOT return a shutdown function (process lifetime management)
//   - Automatic cleanup when the function returns
//   - Server stops on context cancellation or fatal error
//
// Usage:
//
//	func main() {
//	    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	        id, ok := e5s.PeerID(r)
//	        if !ok {
//	            http.Error(w, "unauthorized", http.StatusUnauthorized)
//	            return
//	        }
//	        fmt.Fprintf(w, "Hello %s\n", id)
//	    })
//
//	    if err := e5s.StartSingleThread("e5s.yaml", handler); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// For production deployments with graceful shutdown, use Start() or Serve() instead.
func StartSingleThread(configPath string, handler http.Handler) error {
	srv, identityShutdown, err := buildServer(configPath, handler)
	if err != nil {
		return err
	}
	defer func() {
		// Best effort cleanup - ignore errors during shutdown
		_ = identityShutdown()
	}()

	// Run server in the current goroutine (blocks here)
	err = srv.ListenAndServeTLS("", "")
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server exited with error: %w", err)
	}

	return nil
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
//   - Peer: complete authenticated identity (SPIFFE ID, trust domain, cert expiry)
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
func PeerInfo(r *http.Request) (spiffehttp.Peer, bool) {
	return spiffehttp.PeerFromContext(r.Context())
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
	peer, ok := PeerInfo(r)
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

	// Validate client configuration and get parsed verification policy
	spireConfig, authz, err := config.ValidateClient(&cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid client config: %w", err)
	}

	ctx := context.Background()

	// Centralized SPIRE setup
	x509Source, identityShutdown, err := newSPIRESource(
		ctx,
		cfg.SPIRE.WorkloadSocket,
		spireConfig.InitialFetchTimeout,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create SPIRE source: %w", err)
	}

	// Build client TLS config with server verification
	// Note: cfg.* fields are already validated in config.ValidateClient.
	// spiffehttp still performs its own validation because it is a lower-level API.
	tlsCfg, err := spiffehttp.NewClientTLSConfig(
		ctx,
		x509Source,
		x509Source,
		spiffehttp.ClientConfig{
			ExpectedServerID:          cfg.Client.ExpectedServerSPIFFEID,
			ExpectedServerTrustDomain: cfg.Client.ExpectedServerTrustDomain,
		},
	)
	// Silence unused variable warning
	_ = authz
	if err != nil {
		if shutdownErr := identityShutdown(); shutdownErr != nil {
			return nil, nil, fmt.Errorf("failed to create client TLS config: %w (cleanup error: %v)", err, shutdownErr)
		}
		return nil, nil, fmt.Errorf("failed to create client TLS config: %w", err)
	}

	// Create HTTP client with mTLS
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}

	// identityShutdown is already idempotent, no need for additional sync.Once
	return httpClient, identityShutdown, nil
}
