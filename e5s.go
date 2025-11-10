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
// Server (simple):
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
//	if err := e5s.Serve("e5s.yaml", handler); err != nil {
//	    log.Fatal(err)
//	}
//
// Server (advanced - custom shutdown logic):
//
//	shutdown, err := e5s.Start("e5s.yaml", handler)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer shutdown()
//
// Client (simple):
//
//	err := e5s.WithClient("e5s.yaml", func(client *http.Client) error {
//	    resp, err := client.Get("https://secure-service:8443/api")
//	    if err != nil {
//	        return err
//	    }
//	    defer resp.Body.Close()
//	    // Process response...
//	    return nil
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Client (advanced - long-lived client):
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
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/sufield/e5s/internal/config"
	"github.com/sufield/e5s/spiffehttp"
	"github.com/sufield/e5s/spire"
)

// debugEnabled is set at package initialization based on E5S_DEBUG environment variable.
// Recognized values: "1", "true", "TRUE", "debug", "DEBUG"
var debugEnabled = func() bool {
	v := os.Getenv("E5S_DEBUG")
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "debug")
}()

// debugf logs a debug message with consistent formatting if debug mode is enabled.
func debugf(format string, args ...any) {
	if !debugEnabled {
		return
	}
	log.Printf("e5s DEBUG: "+format, args...)
}

// firstErr returns the first non-nil error from the provided list.
// This is useful for combining multiple cleanup errors during shutdown.
func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

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

// loadServerConfig loads and validates server configuration from the specified file.
// Returns the raw config and validated SPIRE config ready for use.
func loadServerConfig(path string) (config.FileConfig, config.SPIREConfig, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return config.FileConfig{}, config.SPIREConfig{}, fmt.Errorf("failed to load config: %w", err)
	}
	spireCfg, _, err := config.ValidateServer(&cfg)
	if err != nil {
		return config.FileConfig{}, config.SPIREConfig{}, fmt.Errorf("invalid server config: %w", err)
	}
	return cfg, spireCfg, nil
}

// loadClientConfig loads and validates client configuration from the specified file.
// Returns the raw config and validated SPIRE config ready for use.
func loadClientConfig(path string) (config.FileConfig, config.SPIREConfig, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return config.FileConfig{}, config.SPIREConfig{}, fmt.Errorf("failed to load config: %w", err)
	}
	spireCfg, _, err := config.ValidateClient(&cfg)
	if err != nil {
		return config.FileConfig{}, config.SPIREConfig{}, fmt.Errorf("invalid client config: %w", err)
	}
	return cfg, spireCfg, nil
}

// buildServerWithContext constructs the HTTP server and SPIRE identity source with a custom context.
//
// This is the context-aware version used internally by StartWithContext.
// The context is used for SPIRE source initialization and TLS config creation.
func buildServerWithContext(ctx context.Context, configPath string, handler http.Handler) (
	srv *http.Server,
	identityShutdown func() error,
	err error,
) {
	// Load and validate configuration
	cfg, spireConfig, err := loadServerConfig(configPath)
	if err != nil {
		return nil, nil, err
	}

	// Centralized SPIRE setup with provided context
	x509Source, identityShutdown, err := newSPIRESource(
		ctx,
		cfg.SPIRE.WorkloadSocket,
		spireConfig.InitialFetchTimeout,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create SPIRE source: %w", err)
	}

	// Build server TLS config with client verification
	tlsCfg, err := spiffehttp.NewServerTLSConfig(
		ctx,
		x509Source,
		x509Source,
		spiffehttp.ServerConfig{
			AllowedClientID:          cfg.Server.AllowedClientSPIFFEID,
			AllowedClientTrustDomain: cfg.Server.AllowedClientTrustDomain,
		},
	)
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
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Enable debug logging if E5S_DEBUG is set
	if debugEnabled {
		srv.ErrorLog = log.New(os.Stderr, "e5s/http: ", log.LstdFlags|log.Lshortfile)
		debugf("server config_path=%q listen_addr=%q allowed_client_spiffe_id=%q allowed_client_trust_domain=%q",
			configPath,
			cfg.Server.ListenAddr,
			cfg.Server.AllowedClientSPIFFEID,
			cfg.Server.AllowedClientTrustDomain,
		)
	}

	return srv, identityShutdown, nil
}

// buildServer constructs the HTTP server and SPIRE identity source used by both Start and StartSingleThread.
//
// This internal helper factors out common setup logic to ensure both execution modes use
// identical configuration, SPIRE wiring, and TLS setup. It uses context.Background() internally.
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
	return buildServerWithContext(context.Background(), configPath, handler)
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
			shutdownErr = firstErr(err1, err2)
		})
		return shutdownErr
	}

	return shutdownFunc, nil
}

// StartWithContext starts an mTLS server with a custom context for SPIRE initialization.
//
// This is identical to Start() but allows passing a context for more control over
// the SPIRE source initialization and TLS configuration lifecycle.
//
// The context is used for:
//   - SPIRE Workload API connection
//   - TLS configuration setup
//
// Use this when you need:
//   - Parent context cancellation propagation
//   - Custom timeouts during initialization
//   - Integration with existing context hierarchies
//
// Otherwise, use Start() which uses context.Background() internally.
//
// Returns:
//   - shutdown: function to gracefully stop the server and release resources
//   - error: if config loading, SPIRE connection, or server startup fails
//
// Usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	shutdown, err := e5s.StartWithContext(ctx, "e5s.yaml", myHandler)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer shutdown()
func StartWithContext(ctx context.Context, configPath string, handler http.Handler) (shutdown func() error, err error) {
	srv, identityShutdown, err := buildServerWithContext(ctx, configPath, handler)
	if err != nil {
		return nil, err
	}

	// Channel to capture server startup errors
	errCh := make(chan error, 1)

	// Start server in background
	go func() {
		err := srv.ListenAndServeTLS("", "")
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Give server a moment to bind or fail
	select {
	case err := <-errCh:
		if shutdownErr := identityShutdown(); shutdownErr != nil {
			return nil, fmt.Errorf("server startup failed: %w (cleanup error: %v)", err, shutdownErr)
		}
		return nil, fmt.Errorf("server startup failed: %w", err)
	case <-time.After(100 * time.Millisecond):
		// Server started successfully
	}

	// Ensure shutdown is only executed once
	var shutdownOnce sync.Once
	var shutdownErr error

	// Return shutdown function
	shutdownFunc := func() error {
		shutdownOnce.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err1 := srv.Shutdown(ctx)
			err2 := identityShutdown()
			shutdownErr = firstErr(err1, err2)
		})
		return shutdownErr
	}

	return shutdownFunc, nil
}

// Serve starts an mTLS server with graceful shutdown handling.
//
// This is a convenience wrapper around Start() that handles signal management
// and graceful shutdown automatically. It blocks until receiving SIGINT or SIGTERM,
// then performs graceful shutdown with a 5-second timeout.
//
// Use Serve() for typical servers that need standard signal handling.
// Use Start() if you need:
//   - Custom shutdown triggers (not just signals)
//   - Multiple servers or complex lifecycle management
//   - Integration with existing shutdown orchestration
//
// Configuration is identical to Start() - same e5s.yaml format.
//
// The function blocks until:
//   - SIGINT is received (e.g., user presses Ctrl+C)
//   - SIGTERM is received (e.g., Kubernetes pod termination)
//
// Then it:
//   - Stops accepting new connections
//   - Waits up to 5 seconds for in-flight requests to complete
//   - Closes SPIRE resources
//   - Returns any shutdown errors
//
// Usage:
//
//	func main() {
//	    r := chi.NewRouter()
//	    r.Get("/time", timeHandler)
//
//	    if err := e5s.Serve("e5s.yaml", r); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// For debug-friendly single-threaded execution, use StartSingleThread() instead.
func Serve(configPath string, handler http.Handler) error {
	shutdown, err := Start(configPath, handler)
	if err != nil {
		return err
	}
	defer func() {
		if shutdownErr := shutdown(); shutdownErr != nil {
			fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", shutdownErr)
		}
	}()

	// Wait for interrupt signal using context-based cancellation
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	return nil
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

// WithClient creates an mTLS client, executes the provided function, and handles cleanup.
//
// This is a convenience wrapper around Client() that manages the client lifecycle
// automatically. It creates the client, executes your function, and ensures cleanup
// happens even if the function panics or returns an error.
//
// Use WithClient() for typical request patterns where you don't need long-lived clients.
// Use Client() if you need:
//   - Long-lived clients making multiple requests over time
//   - Manual control over client lifetime
//   - Sharing a client across multiple goroutines
//
// Configuration is identical to Client() - same e5s.yaml format.
//
// The function executes synchronously and returns any error from:
//   - Client creation
//   - The callback function
//   - Cleanup (logged but not returned unless callback succeeded)
//
// Usage:
//
//	err := e5s.WithClient("e5s.yaml", func(client *http.Client) error {
//	    resp, err := client.Get("https://secure-service:8443/api")
//	    if err != nil {
//	        return err
//	    }
//	    defer resp.Body.Close()
//
//	    body, err := io.ReadAll(resp.Body)
//	    if err != nil {
//	        return err
//	    }
//	    fmt.Println(string(body))
//	    return nil
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
func WithClient(configPath string, fn func(*http.Client) error) error {
	client, cleanup, err := Client(configPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "Client cleanup error: %v\n", err)
		}
	}()
	return fn(client)
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
	// Load and validate configuration
	cfg, spireConfig, err := loadClientConfig(configPath)
	if err != nil {
		return nil, nil, err
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

	// Enable debug logging if E5S_DEBUG is set
	debugf("client config_path=%q expected_server_spiffe_id=%q expected_server_trust_domain=%q",
		configPath,
		cfg.Client.ExpectedServerSPIFFEID,
		cfg.Client.ExpectedServerTrustDomain,
	)

	// identityShutdown is already idempotent, no need for additional sync.Once
	return httpClient, identityShutdown, nil
}

// ClientWithContext returns an HTTP client configured for mTLS using SPIRE with a custom context.
//
// This is identical to Client() but allows passing a context for more control over
// the SPIRE source initialization and TLS configuration lifecycle.
//
// The context is used for:
//   - SPIRE Workload API connection
//   - TLS configuration setup
//
// Use this when you need:
//   - Parent context cancellation propagation
//   - Custom timeouts during initialization
//   - Integration with existing context hierarchies
//
// Otherwise, use Client() which uses context.Background() internally.
//
// Returns:
//   - client: HTTP client ready for mTLS connections
//   - shutdown: function to release SPIRE resources (should be deferred)
//   - error: if config loading, SPIRE connection, or TLS setup fails
//
// Usage:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	client, shutdown, err := e5s.ClientWithContext(ctx, "e5s.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer shutdown()
func ClientWithContext(ctx context.Context, configPath string) (*http.Client, func() error, error) {
	// Load and validate configuration
	cfg, spireConfig, err := loadClientConfig(configPath)
	if err != nil {
		return nil, nil, err
	}

	// Centralized SPIRE setup with provided context
	x509Source, identityShutdown, err := newSPIRESource(
		ctx,
		cfg.SPIRE.WorkloadSocket,
		spireConfig.InitialFetchTimeout,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create SPIRE source: %w", err)
	}

	// Build client TLS config with server verification
	tlsCfg, err := spiffehttp.NewClientTLSConfig(
		ctx,
		x509Source,
		x509Source,
		spiffehttp.ClientConfig{
			ExpectedServerID:          cfg.Client.ExpectedServerSPIFFEID,
			ExpectedServerTrustDomain: cfg.Client.ExpectedServerTrustDomain,
		},
	)
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

	// Enable debug logging if E5S_DEBUG is set
	debugf("client config_path=%q expected_server_spiffe_id=%q expected_server_trust_domain=%q",
		configPath,
		cfg.Client.ExpectedServerSPIFFEID,
		cfg.Client.ExpectedServerTrustDomain,
	)

	return httpClient, identityShutdown, nil
}
