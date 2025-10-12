package httpapi

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pocket/hexagon/spire/internal/config"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffetls"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// X509SourceProvider defines how to obtain an X509Source
// Implementations can create from SPIRE Workload API (production) or in-memory (testing)
type X509SourceProvider interface {
	// GetX509Source returns an X509Source for mTLS configuration
	GetX509Source(ctx context.Context) (x509svid.Source, x509bundle.Source, io.Closer, error)
}

// WorkloadAPISourceProvider creates X509Source from SPIRE Workload API (production)
type WorkloadAPISourceProvider struct {
	SocketPath string
}

// GetX509Source implements X509SourceProvider for production SPIRE Workload API
func (p *WorkloadAPISourceProvider) GetX509Source(ctx context.Context) (x509svid.Source, x509bundle.Source, io.Closer, error) {
	x509Source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(
			workloadapi.WithAddr(p.SocketPath),
		),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create X509Source: %w", err)
	}
	// workloadapi.X509Source implements both x509svid.Source and x509bundle.Source
	return x509Source, x509Source, x509Source, nil
}

// InMemorySourceProvider wraps in-memory X509Source (testing/development)
type InMemorySourceProvider struct {
	SVIDSource   x509svid.Source
	BundleSource x509bundle.Source
}

// GetX509Source implements X509SourceProvider for in-memory mode
func (p *InMemorySourceProvider) GetX509Source(ctx context.Context) (x509svid.Source, x509bundle.Source, io.Closer, error) {
	// In-memory sources don't need cleanup
	return p.SVIDSource, p.BundleSource, io.NopCloser(nil), nil
}

// ServerConfig contains configuration for creating an HTTP server with mTLS.
type ServerConfig struct {
	// Address is the server listen address (e.g., ":8443")
	Address string

	// X509SourceProvider provides the X509Source (production or in-memory)
	X509SourceProvider X509SourceProvider

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
	closer     io.Closer
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
	if cfg.X509SourceProvider == nil {
		return nil, fmt.Errorf("X509SourceProvider is required")
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

	// Get X509Source from provider (production or in-memory)
	svidSource, bundleSource, closer, err := cfg.X509SourceProvider.GetX509Source(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get X509Source: %w", err)
	}

	// Create mTLS server configuration
	// - Server presents its SVID to clients
	// - Clients must present valid SVIDs
	// - Authorizer verifies client identity (authentication only)
	tlsConfig := tlsconfig.MTLSServerConfig(
		svidSource,     // SVID source (server certificate)
		bundleSource,   // Bundle source (trusted CAs)
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
		closer:     closer,
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
	if s.closer != nil {
		if err := s.closer.Close(); err != nil {
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
