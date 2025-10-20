package identityserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"

	"github.com/pocket/hexagon/spire/internal/config"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Sentinel errors for identity operations.
var (
	// ErrNoSPIFFEID indicates no SPIFFE ID is present in the request context.
	ErrNoSPIFFEID = errors.New("SPIFFE ID not found in request context")
	// ErrCannotRegisterAfterStart indicates handler registration attempted after Start().
	ErrCannotRegisterAfterStart = errors.New("cannot register handler after Start")
)

// spiffeIDKey is a zero-sized type for context keys to prevent collisions.
type spiffeIDKey struct{}

// spiffeIDCtxKey is the context key for storing the authenticated client identity.
var spiffeIDCtxKey spiffeIDKey

// spiffeServer implements ports.MTLSServer using go-spiffe SDK
type spiffeServer struct {
	cfg    ports.MTLSConfig
	source *workloadapi.X509Source
	srv    *http.Server
	mux    *http.ServeMux
	ln     net.Listener // Pre-bound listener

	mu        sync.Mutex
	started   bool      // Start() called
	stopped   bool      // Shutdown() completed
	closed    bool      // Close() completed
	startOnce sync.Once // Ensures Start() runs once
}

// New creates a new mTLS HTTP server that authenticates clients.
// Returns a server that serves HTTPS using identity-based authentication.
func New(ctx context.Context, cfg *ports.MTLSConfig) (ports.MTLSServer, error) {
	// Validate required configuration
	if cfg.WorkloadAPI.SocketPath == "" {
		return nil, fmt.Errorf("workload api socket path is required")
	}
	// Require exactly one authorization policy (not both)
	if cfg.SPIFFE.AllowedPeerID == "" && cfg.SPIFFE.AllowedTrustDomain == "" {
		return nil, fmt.Errorf("at least one SPIFFE authorization policy required (AllowedPeerID or AllowedTrustDomain)")
	}
	if cfg.SPIFFE.AllowedPeerID != "" && cfg.SPIFFE.AllowedTrustDomain != "" {
		return nil, fmt.Errorf("configure either AllowedPeerID or AllowedTrustDomain, not both")
	}

	// Apply defaults (adapter responsibility, not in ports)
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
			if closeErr := source.Close(); closeErr != nil {
				return nil, fmt.Errorf("parse allowed peer ID: %w (close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("parse allowed peer ID: %w", err)
		}
		authorizer = tlsconfig.AuthorizeID(clientID)
	} else if cfg.SPIFFE.AllowedTrustDomain != "" {
		// Authorize any ID in trust domain (SDK handles canonicalization)
		trustDomain, err := spiffeid.TrustDomainFromString(cfg.SPIFFE.AllowedTrustDomain)
		if err != nil {
			if closeErr := source.Close(); closeErr != nil {
				return nil, fmt.Errorf("parse allowed trust domain: %w (close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("parse allowed trust domain: %w", err)
		}
		authorizer = tlsconfig.AuthorizeMemberOf(trustDomain)
	}

	// mTLS server config: present our SVID, verify client identity
	tlsCfg := tlsconfig.MTLSServerConfig(source, source, authorizer)
	// Apply TLS hardening
	tlsCfg.MinVersion = tls.VersionTLS13 // TLS 1.3
	tlsCfg.CipherSuites = nil            // nil = use Go's secure defaults (TLS 1.3 ignores this)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           mux,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	return &spiffeServer{
		cfg:    *cfg,
		source: source,
		srv:    server,
		mux:    mux,
	}, nil
}

// Handle registers an HTTP handler with automatic SPIFFE ID extraction.
// Must be called before Start().
// Returns ErrCannotRegisterAfterStart if called after Start().
func (s *spiffeServer) Handle(pattern string, handler http.Handler) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return ErrCannotRegisterAfterStart
	}
	s.mux.Handle(pattern, s.wrapHandler(handler))
	return nil
}

// Start begins serving HTTPS with identity-based mTLS.
// Blocks until the server shuts down (graceful or error).
// Pre-binds the listener to detect port conflicts immediately before blocking.
func (s *spiffeServer) Start(ctx context.Context) error {
	var startErr error
	s.startOnce.Do(func() {
		// Pre-bind the listener to detect port conflicts immediately
		ln, err := net.Listen("tcp", s.cfg.HTTP.Address)
		if err != nil {
			startErr = fmt.Errorf("bind failed: %w", err)
			return
		}

		s.mu.Lock()
		s.started = true
		s.ln = ln
		s.mu.Unlock()

		// Serve TLS - blocks until shutdown
		if err := s.srv.ServeTLS(ln, "", ""); err != nil && err != http.ErrServerClosed {
			startErr = fmt.Errorf("serve: %w", err)
			return
		}
	})
	return startErr
}

// Shutdown gracefully stops the HTTP server, waiting for active connections.
// After Shutdown, you must call Close() to release resources (X509Source).
func (s *spiffeServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return nil // Already stopped
	}

	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	s.stopped = true
	return nil
}

// Close releases resources (X509Source).
// Idempotent - safe to call multiple times.
// Call this after Shutdown() to properly clean up.
func (s *spiffeServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil // Already closed
	}

	s.closed = true

	if s.source != nil {
		if err := s.source.Close(); err != nil {
			return fmt.Errorf("close X509Source: %w", err)
		}
	}

	return nil
}

// wrapHandler adds identity extraction middleware to the handler.
// Translates SPIFFE ID from TLS connection → port-level ports.Identity.
func (s *spiffeServer) wrapHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify TLS connection
		if r.TLS == nil {
			http.Error(w, "TLS connection required", http.StatusBadRequest)
			return
		}

		// Extract peer SPIFFE ID from TLS connection (adapter-specific)
		peerID, err := spiffetls.PeerIDFromConnectionState(*r.TLS)
		if err != nil {
			http.Error(w, "Failed to get peer identity", http.StatusUnauthorized)
			return
		}

		// Translate SDK type → port-level Identity (adapter responsibility)
		identity := ports.Identity{
			SPIFFEID:    peerID.String(),
			TrustDomain: peerID.TrustDomain().String(),
			Path:        peerID.Path(),
		}

		// Inject port-level identity into context (handlers depend on ports, not adapter)
		ctx := ports.WithIdentity(r.Context(), identity)

		// Also keep SDK-specific ID for internal adapter use (backward compat)
		ctx = context.WithValue(ctx, spiffeIDCtxKey, peerID)

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ContextWithIdentity adds a SPIFFE ID to the context.
// Useful for testing or propagating identity in non-HTTP code.
func ContextWithIdentity(ctx context.Context, id spiffeid.ID) context.Context {
	return context.WithValue(ctx, spiffeIDCtxKey, id)
}

// IdentityFromContext extracts the SPIFFE ID from the context.
// Returns the identity and true if present and non-zero, zero value and false otherwise.
// Treats zero-value IDs as missing. Returns false if context is nil.
func IdentityFromContext(ctx context.Context) (spiffeid.ID, bool) {
	if ctx == nil {
		return spiffeid.ID{}, false
	}
	id, ok := ctx.Value(spiffeIDCtxKey).(spiffeid.ID)
	if !ok || id.IsZero() {
		return spiffeid.ID{}, false
	}
	return id, true
}

// GetIdentity extracts the authenticated client identity from the request.
// Returns the identity and true if present and non-zero, zero value and false otherwise.
// Treats zero-value IDs as missing. Returns false if request is nil.
func GetIdentity(r *http.Request) (spiffeid.ID, bool) {
	if r == nil {
		return spiffeid.ID{}, false
	}
	return IdentityFromContext(r.Context())
}

// RequireIdentity extracts the authenticated client identity from the request.
// Returns ErrNoSPIFFEID if the identity is not present, is zero-valued, or request is nil.
func RequireIdentity(r *http.Request) (spiffeid.ID, error) {
	id, ok := GetIdentity(r)
	if !ok {
		return spiffeid.ID{}, ErrNoSPIFFEID
	}
	return id, nil
}
