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
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sufield/e5s/internal/config"
	"github.com/sufield/e5s/pkg/spiffehttp"
	"github.com/sufield/e5s/pkg/spire"
)

// getConfigPath returns the config file path using intelligent defaults:
//  1. Check E5S_CONFIG environment variable
//  2. Fall back to "e5s.yaml" in current directory
//
// This allows zero-configuration usage in most cases while supporting
// environment-specific overrides in production.
func getConfigPath() string {
	if path := os.Getenv("E5S_CONFIG"); path != "" {
		return path
	}
	return "e5s.yaml"
}

// Run starts an mTLS server with zero configuration required.
//
// This is the simplest way to run an mTLS server. It automatically handles:
//   - Config file discovery (E5S_CONFIG env var or e5s.yaml)
//   - SPIRE connection and mTLS setup
//   - Signal handling (SIGINT/SIGTERM)
//   - Graceful shutdown
//
// The server will run until interrupted (Ctrl+C or kill signal).
//
// For more control over signal handling, use StartServer() instead.
// For explicit config paths, use Start() instead.
//
// Configuration (e5s.yaml or path from E5S_CONFIG):
//
//	spire:
//	  workload_socket: unix:///tmp/spire-agent/public/api.sock
//	server:
//	  listen_addr: ":8443"
//	  allowed_client_trust_domain: "example.org"
//
// Usage:
//
//	func main() {
//	    r := chi.NewRouter()
//	    r.Get("/hello", func(w http.ResponseWriter, req *http.Request) {
//	        id, ok := e5s.PeerID(req)
//	        if !ok {
//	            http.Error(w, "unauthorized", http.StatusUnauthorized)
//	            return
//	        }
//	        fmt.Fprintf(w, "Hello, %s!\n", id)
//	    })
//	    e5s.Run(r)
//	}
//
// This function will block until the server receives SIGINT or SIGTERM.
func Run(handler http.Handler) {
	// Create context that listens for interrupt signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	// Start server with intelligent config defaults
	shutdown, err := StartServer(handler)
	if err != nil {
		stop() // Clean up signal handler before exit
		log.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	log.Println("Server running - press Ctrl+C to stop")

	// Wait for interrupt signal for graceful shutdown
	<-ctx.Done()
	stop()
	log.Println("Shutting down gracefully...")
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
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Validate server configuration and get parsed authorization policy
	spireConfig, authz, err := config.ValidateServer(&cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid server config: %w", err)
	}

	ctx := context.Background()

	// Connect to SPIRE Workload API with timeout for initial fetch
	source, err := spire.NewSource(ctx, spire.Config{
		WorkloadSocket:      cfg.SPIRE.WorkloadSocket,
		InitialFetchTimeout: spireConfig.InitialFetchTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE source: %w", err)
	}

	// Get SDK X509Source for TLS config
	x509Source := source.X509Source()

	// Build server TLS config with client verification
	// Note: We pass the pre-validated strings to spiffehttp (which will parse them again).
	// This is acceptable because spiffehttp is a lower-level library that users can call
	// directly, so it must handle its own validation. The benefit of our validation is:
	// 1. Input trimming for robustness
	// 2. Early error detection with clear messages
	// 3. Parsed values available if needed (currently unused but available via authz)
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
		if closeErr := source.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to create server TLS config: %w (cleanup error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to create server TLS config: %w", err)
	}

	// Wrap handler to inject peer identity into request context
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if peer, ok := spiffehttp.PeerFromRequest(r); ok {
			r = r.WithContext(spiffehttp.WithPeer(r.Context(), peer))
		}
		handler.ServeHTTP(w, r)
	})

	// Create HTTP server with mTLS
	srv := &http.Server{
		Addr:              cfg.Server.ListenAddr,
		Handler:           wrapped,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
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
		if closeErr := source.Close(); closeErr != nil {
			return nil, fmt.Errorf("server startup failed: %w (cleanup error: %v)", err, closeErr)
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

// StartServer starts a production-grade mTLS server with intelligent defaults.
//
// This is the recommended API for most users. It automatically:
//   - Checks E5S_CONFIG environment variable for config file path
//   - Falls back to "e5s.yaml" in current directory
//   - Loads configuration and connects to SPIRE
//   - Starts mTLS server with automatic certificate rotation
//
// For explicit config file paths, use Start(configPath, handler) instead.
//
// Configuration (e5s.yaml or path from E5S_CONFIG):
//
//	spire:
//	  workload_socket: unix:///tmp/spire-agent/public/api.sock
//	server:
//	  listen_addr: ":8443"
//	  allowed_client_trust_domain: "example.org"
//
// Usage:
//
//	shutdown, err := e5s.StartServer(myHandler)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer shutdown()
//
// See Start() documentation for full details on server behavior and configuration.
func StartServer(handler http.Handler) (shutdown func() error, err error) {
	return Start(getConfigPath(), handler)
}

// NewClient returns an HTTP client configured for mTLS with intelligent defaults.
//
// This is the recommended API for most users. It automatically:
//   - Checks E5S_CONFIG environment variable for config file path
//   - Falls back to "e5s.yaml" in current directory
//   - Loads configuration and connects to SPIRE
//   - Configures mTLS client with automatic certificate rotation
//
// For explicit config file paths, use Client(configPath) instead.
//
// Configuration (e5s.yaml or path from E5S_CONFIG):
//
//	spire:
//	  workload_socket: unix:///tmp/spire-agent/public/api.sock
//	client:
//	  expected_server_trust_domain: "example.org"
//
// Usage:
//
//	client, shutdown, err := e5s.NewClient()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer shutdown()
//
//	resp, err := client.Get("https://secure-service:8443/api")
//
// See Client() documentation for full details on client behavior and configuration.
func NewClient() (*http.Client, func() error, error) {
	return Client(getConfigPath())
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
	peer, ok := spiffehttp.PeerFromContext(r.Context())
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

	// Connect to SPIRE Workload API with timeout for initial fetch
	source, err := spire.NewSource(ctx, spire.Config{
		WorkloadSocket:      cfg.SPIRE.WorkloadSocket,
		InitialFetchTimeout: spireConfig.InitialFetchTimeout,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create SPIRE source: %w", err)
	}

	// Get SDK X509Source for TLS config
	x509Source := source.X509Source()

	// Build client TLS config with server verification
	// Note: We pass the pre-validated strings to spiffehttp (which will parse them again).
	// This is acceptable because spiffehttp is a lower-level library that users can call
	// directly, so it must handle its own validation. The benefit of our validation is:
	// 1. Input trimming for robustness
	// 2. Early error detection with clear messages
	// 3. Parsed values available if needed (currently unused but available via authz)
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
		if closeErr := source.Close(); closeErr != nil {
			return nil, nil, fmt.Errorf("failed to create client TLS config: %w (cleanup error: %v)", err, closeErr)
		}
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

// bodyCloser wraps an io.ReadCloser to call an additional cleanup function on Close.
// This allows automatic resource cleanup when the response body is closed.
type bodyCloser struct {
	io.ReadCloser
	cleanup func()
}

func (b *bodyCloser) Close() error {
	err := b.ReadCloser.Close()
	b.cleanup()
	return err
}

// Get performs an mTLS GET request with zero configuration required.
//
// This is the simplest way to make an mTLS request. It automatically:
//   - Discovers config file (E5S_CONFIG env var or e5s.yaml)
//   - Creates mTLS client with SPIRE connection
//   - Performs the GET request
//   - Ties cleanup to response body closure
//
// Shutdown is automatically handled when you close the response body.
// This ensures proper resource cleanup without requiring explicit shutdown calls.
//
// Configuration (e5s.yaml or path from E5S_CONFIG):
//
//	spire:
//	  workload_socket: unix:///tmp/spire-agent/public/api.sock
//	client:
//	  expected_server_trust_domain: "example.org"
//
// Usage:
//
//	func main() {
//	    resp, err := e5s.Get("https://api.example.com:8443/data")
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer resp.Body.Close()  // This also triggers cleanup
//
//	    body, _ := io.ReadAll(resp.Body)
//	    fmt.Println(string(body))
//	}
//
// For making multiple requests with the same client, use NewClient() instead.
// For explicit config paths, use Client() instead.
//
// Returns:
//   - response: HTTP response with wrapped body that handles cleanup
//   - error: if config loading, SPIRE connection, or request fails
func Get(url string) (*http.Response, error) {
	// Create client with intelligent defaults
	client, shutdown, err := NewClient()
	if err != nil {
		return nil, err
	}

	// Perform GET request
	resp, err := client.Get(url)
	if err != nil {
		if shutdownErr := shutdown(); shutdownErr != nil {
			log.Printf("Error during shutdown: %v", shutdownErr)
		}
		return nil, err
	}

	// Wrap response body to trigger cleanup on close
	resp.Body = &bodyCloser{
		ReadCloser: resp.Body,
		cleanup: func() {
			if shutdownErr := shutdown(); shutdownErr != nil {
				log.Printf("Error during shutdown: %v", shutdownErr)
			}
		},
	}

	return resp, nil
}
