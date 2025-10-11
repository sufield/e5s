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

// spiffeIDKey is the context key for storing SPIFFE ID
type contextKey string

const spiffeIDKey contextKey = "spiffe-id"

// spiffeServer implements ports.MTLSServer using go-spiffe SDK
type spiffeServer struct {
	cfg    ports.MTLSConfig
	source *workloadapi.X509Source
	srv    *http.Server
	mux    *http.ServeMux
	once   sync.Once
	closed bool
	mu     sync.Mutex
}

// NewSPIFFEServer returns a Server that authenticates clients via SPIFFE ID
// and serves HTTPS using the Workload API-provided SVID.
func NewSPIFFEServer(ctx context.Context, cfg ports.MTLSConfig) (ports.MTLSServer, error) {
	// Validate required configuration
	if cfg.WorkloadAPI.SocketPath == "" {
		return nil, fmt.Errorf("workload api socket path is required")
	}
	if cfg.SPIFFE.AllowedPeerID == "" {
		return nil, fmt.Errorf("spiffe allowed peer id is required")
	}

	// Apply defaults
	if cfg.HTTP.Address == "" {
		cfg.HTTP.Address = config.DefaultHTTPAddress
	}
	if cfg.HTTP.ReadHeaderTimeout <= 0 {
		cfg.HTTP.ReadHeaderTimeout = config.DefaultReadHeaderTimeout
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

	// Parse the client's expected SPIFFE ID
	clientID, err := spiffeid.FromString(cfg.SPIFFE.AllowedPeerID)
	if err != nil {
		source.Close()
		return nil, fmt.Errorf("parse allowed peer ID: %w", err)
	}

	// mTLS server config: present our SVID, verify client ID
	tlsCfg := tlsconfig.MTLSServerConfig(source, source, tlsconfig.AuthorizeID(clientID))

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           mux,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
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

// Start begins serving HTTPS with identity-based mTLS
func (s *spiffeServer) Start(ctx context.Context) error {
	var startErr error
	s.once.Do(func() {
		go func() {
			log.Printf("Starting mTLS server on %s", s.cfg.HTTP.Address)
			if err := s.srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Printf("Server error: %v", err)
			}
		}()
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.Shutdown(shutdownCtx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
	})
	return startErr
}

// Shutdown gracefully stops the server
func (s *spiffeServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	log.Println("Shutting down server...")
	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	return nil
}

// Close releases resources (X309Source, connections, etc.)
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

// GetMux returns the underlying ServeMux for advanced use cases
func (s *spiffeServer) GetMux() *http.ServeMux {
	return s.mux
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

// GetSPIFFEID extracts the SPIFFE ID from the request context
func GetSPIFFEID(r *http.Request) (spiffeid.ID, bool) {
	id, ok := r.Context().Value(spiffeIDKey).(spiffeid.ID)
	return id, ok
}

// MustGetSPIFFEID extracts the SPIFFE ID or panics if not present
func MustGetSPIFFEID(r *http.Request) spiffeid.ID {
	id, ok := GetSPIFFEID(r)
	if !ok {
		panic("SPIFFE ID not found in request context")
	}
	return id
}
