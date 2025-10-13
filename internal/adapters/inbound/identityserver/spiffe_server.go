package identityserver

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pocket/hexagon/spire/internal/config"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// contextKey is the type for context keys to prevent collisions.
type contextKey string

// spiffeIDKey is the context key for storing the authenticated client identity.
const spiffeIDKey contextKey = "spiffe-id"

// spiffeServer implements ports.MTLSServer using go-spiffe SDK
type spiffeServer struct {
	cfg       ports.MTLSConfig
	source    *workloadapi.X509Source
	srv       *http.Server
	mux       *http.ServeMux
	startOnce sync.Once
	startErr  error
	closed    bool
	mu        sync.Mutex
}

// New creates a new mTLS HTTP server that authenticates clients.
// Returns a server that serves HTTPS using identity-based authentication.
func New(ctx context.Context, cfg ports.MTLSConfig) (ports.MTLSServer, error) {
	// Validate required configuration
	if cfg.WorkloadAPI.SocketPath == "" {
		return nil, fmt.Errorf("workload api socket path is required")
	}
	// Require at least one authorization policy
	if cfg.SPIFFE.AllowedPeerID == "" && cfg.SPIFFE.AllowedTrustDomain == "" {
		return nil, fmt.Errorf("at least one SPIFFE authorization policy required (AllowedPeerID or AllowedTrustDomain)")
	}

	// Apply defaults
	if cfg.HTTP.Address == "" {
		cfg.HTTP.Address = config.DefaultHTTPAddress
	}
	if cfg.HTTP.ReadHeaderTimeout <= 0 {
		cfg.HTTP.ReadHeaderTimeout = config.DefaultReadHeaderTimeout
	}
	if cfg.HTTP.ReadTimeout <= 0 {
		cfg.HTTP.ReadTimeout = config.DefaultReadTimeout
	}
	if cfg.HTTP.WriteTimeout <= 0 {
		cfg.HTTP.WriteTimeout = config.DefaultWriteTimeout
	}
	if cfg.HTTP.IdleTimeout <= 0 {
		cfg.HTTP.IdleTimeout = config.DefaultIdleTimeout
	}
	if cfg.HTTP.ShutdownTimeout <= 0 {
		cfg.HTTP.ShutdownTimeout = 10 * time.Second
	}
	if cfg.HTTP.MaxHeaderBytes <= 0 {
		cfg.HTTP.MaxHeaderBytes = 1 << 20 // 1 MB default
	}

	// Build the X509 source from the local SPIRE Agent
	source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(workloadapi.WithAddr(cfg.WorkloadAPI.SocketPath)),
	)
	if err != nil {
		return nil, fmt.Errorf("create X509Source: %w", err)
	}

	// Build authorization policy based on configuration
	var authorizer tlsconfig.Authorizer
	if cfg.SPIFFE.AllowedPeerID != "" {
		// Authorize specific SPIFFE ID (exact match)
		clientID, err := spiffeid.FromString(cfg.SPIFFE.AllowedPeerID)
		if err != nil {
			source.Close()
			return nil, fmt.Errorf("parse allowed peer ID: %w", err)
		}
		authorizer = tlsconfig.AuthorizeID(clientID)
	} else if cfg.SPIFFE.AllowedTrustDomain != "" {
		// Authorize any ID in trust domain
		trustDomain, err := spiffeid.TrustDomainFromString(cfg.SPIFFE.AllowedTrustDomain)
		if err != nil {
			source.Close()
			return nil, fmt.Errorf("parse allowed trust domain: %w", err)
		}
		authorizer = tlsconfig.AuthorizeMemberOf(trustDomain)
	}

	// mTLS server config: present our SVID, verify client identity
	tlsCfg := tlsconfig.MTLSServerConfig(source, source, authorizer)
	// Apply TLS hardening
	tlsCfg.MinVersion = 0x0304 // TLS 1.3
	tlsCfg.CipherSuites = nil  // nil = use Go's secure defaults (TLS 1.3 ignores this)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           mux,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}

	return &spiffeServer{
		cfg:    cfg,
		source: source,
		srv:    server,
		mux:    mux,
	}, nil
}

// Handle registers an HTTP handler with automatic SPIFFE ID extraction
func (s *spiffeServer) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, s.wrapHandler(handler))
}

// HandleFunc registers a function handler with automatic SPIFFE ID extraction (convenience method)
func (s *spiffeServer) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	s.mux.Handle(pattern, s.wrapHandler(http.HandlerFunc(handler)))
}

// Start begins serving HTTPS with identity-based mTLS.
// Returns immediately after starting the server in a goroutine.
// Startup errors (port bind failures) are captured and returned synchronously.
// Use Shutdown() to gracefully stop the server.
func (s *spiffeServer) Start(ctx context.Context) error {
	s.startOnce.Do(func() {
		// Create a channel to capture startup errors (buffer size 1 for non-blocking send)
		errCh := make(chan error, 1)

		go func() {
			log.Printf("Starting mTLS server on %s", s.cfg.HTTP.Address)
			if err := s.srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				// Send error to channel (non-blocking if already closed)
				select {
				case errCh <- err:
				default:
				}
				log.Printf("Server error: %v", err)
			}
		}()

		// Wait briefly for startup errors (port binding failures happen immediately)
		select {
		case err := <-errCh:
			s.startErr = fmt.Errorf("server startup failed: %w", err)
		case <-time.After(100 * time.Millisecond):
			// No error within 100ms, assume successful startup
			s.startErr = nil
		}
	})
	return s.startErr
}

// Shutdown gracefully stops the server, waiting for active connections.
// Uses ShutdownTimeout from config if ctx doesn't have its own deadline.
func (s *spiffeServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	log.Println("Shutting down server...")

	// If ctx has no deadline, apply ShutdownTimeout from config
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.cfg.HTTP.ShutdownTimeout)
		defer cancel()
	}

	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	s.closed = true
	return nil
}

// Close releases resources (X509Source, connections, etc.)
func (s *spiffeServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	if s.source != nil {
		if err := s.source.Close(); err != nil {
			return fmt.Errorf("close X509Source: %w", err)
		}
	}

	log.Println("Server resources released")
	return nil
}

// wrapHandler adds SPIFFE ID extraction middleware to the handler
func (s *spiffeServer) wrapHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify TLS connection
		if r.TLS == nil {
			http.Error(w, "TLS connection required", http.StatusBadRequest)
			return
		}

		// Extract peer SPIFFE ID from TLS connection
		peerID, err := spiffetls.PeerIDFromConnectionState(*r.TLS)
		if err != nil {
			http.Error(w, "Failed to get peer identity", http.StatusUnauthorized)
			log.Printf("Failed to extract peer ID: %v", err)
			return
		}

		// Add SPIFFE ID to request context
		ctx := context.WithValue(r.Context(), spiffeIDKey, peerID)
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetIdentity extracts the authenticated client identity from the request.
// Returns the identity and true if present, zero value and false otherwise.
func GetIdentity(r *http.Request) (spiffeid.ID, bool) {
	id, ok := r.Context().Value(spiffeIDKey).(spiffeid.ID)
	return id, ok
}

// MustGetIdentity extracts the identity or panics if not present.
// Use only in handlers where authentication is guaranteed.
func MustGetIdentity(r *http.Request) spiffeid.ID {
	id, ok := GetIdentity(r)
	if !ok {
		panic("identity not found in request context")
	}
	return id
}
