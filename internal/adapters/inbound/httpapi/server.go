package httpapi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pocket/hexagon/spire/internal/config"
	"github.com/spiffe/go-spiffe/v2/spiffetls"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// ServerConfig contains configuration for creating an HTTP server with mTLS.
type ServerConfig struct {
	// Address is the server listen address (e.g., ":8443")
	Address string

	// SocketPath is the SPIRE agent socket path (e.g., "unix:///tmp/spire-agent/public/api.sock")
	SocketPath string

	// Authorizer verifies client identities (use tlsconfig.AuthorizeAny(), AuthorizeID(), etc.)
	Authorizer tlsconfig.Authorizer

	// Timeouts (optional - defaults provided)
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// HTTPServer provides an mTLS HTTP server that authenticates clients using X.509 SVIDs.
type HTTPServer struct {
	server     *http.Server
	x509Source *workloadapi.X509Source
	authorizer tlsconfig.Authorizer
	mux        *http.ServeMux
	once       sync.Once
	mu         sync.Mutex
	closed     bool
}

// NewHTTPServer creates an mTLS HTTP server that authenticates clients.
func NewHTTPServer(ctx context.Context, cfg ServerConfig) (*HTTPServer, error) {
	// Validate required fields
	if cfg.Address == "" {
		return nil, fmt.Errorf("address is required")
	}
	if cfg.SocketPath == "" {
		return nil, fmt.Errorf("socket path is required")
	}
	if cfg.Authorizer == nil {
		return nil, fmt.Errorf("authorizer is required")
	}

	// Apply defaults for timeouts
	if cfg.ReadHeaderTimeout == 0 {
		cfg.ReadHeaderTimeout = config.DefaultReadHeaderTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = config.DefaultReadTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = config.DefaultWriteTimeout
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = config.DefaultIdleTimeout
	}

	// Create X.509 source from SPIRE Workload API
	// This handles automatic SVID rotation
	x509Source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(
			workloadapi.WithAddr(cfg.SocketPath),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create X509Source: %w", err)
	}

	// Create mTLS server configuration
	// - Server presents its SVID to clients
	// - Clients must present valid SVIDs
	// - Authorizer verifies client identity (authentication only)
	tlsConfig := tlsconfig.MTLSServerConfig(
		x509Source,     // SVID source (server certificate)
		x509Source,     // Bundle source (trusted CAs)
		cfg.Authorizer, // Identity verification (go-spiffe only)
	)

	mux := http.NewServeMux()

	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           mux,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	return &HTTPServer{
		server:     server,
		x509Source: x509Source,
		authorizer: cfg.Authorizer,
		mux:        mux,
	}, nil
}

// Start starts the mTLS HTTP server.
func (s *HTTPServer) Start(ctx context.Context) error {
	s.once.Do(func() {
		go func() {
			log.Printf("Starting mTLS server on %s", s.server.Addr)
			if err := s.server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Printf("Server error: %v", err)
			}
		}()
	})
	// Wait briefly to catch immediate startup errors
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		log.Printf("mTLS server listening on %s", s.server.Addr)
		return nil
	}
}

// Shutdown gracefully shuts down the server without closing X509Source.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	log.Println("Shutting down server...")
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the server and releases all resources.
func (s *HTTPServer) Stop(ctx context.Context) error {
	// First shutdown the server
	if err := s.Shutdown(ctx); err != nil {
		return err
	}
	// Then close resources
	return s.Close()
}

// Close releases resources (X509Source, connections, etc.).
func (s *HTTPServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.x509Source != nil {
		if err := s.x509Source.Close(); err != nil {
			return fmt.Errorf("close X509Source: %w", err)
		}
	}
	log.Println("Server resources released")
	return nil
}

// RegisterHandler registers an HTTP handler for the given pattern.
// The handler is wrapped with middleware to extract and expose the client's SPIFFE ID.
func (s *HTTPServer) RegisterHandler(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, s.wrapHandler(handler))
}

// wrapHandler adds SPIFFE ID extraction middleware to the handler.
func (s *HTTPServer) wrapHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract client SPIFFE ID from TLS connection
		// This is the authenticated identity - application decides what to do with it
		if r.TLS == nil {
			http.Error(w, "TLS connection required", http.StatusBadRequest)
			return
		}

		peerID, err := spiffetls.PeerIDFromConnectionState(*r.TLS)
		if err != nil {
			http.Error(w, "Failed to get peer identity", http.StatusUnauthorized)
			log.Printf("failed to extract peer ID: %v", err)
			return
		}

		// Add SPIFFE ID to request context for handler use
		ctx := context.WithValue(r.Context(), spiffeIDKey, peerID)
		handler(w, r.WithContext(ctx))
	}
}

// GetMux returns the underlying ServeMux for advanced usage.
// Use RegisterHandler for typical handler registration.
func (s *HTTPServer) GetMux() *http.ServeMux {
	return s.mux
}
