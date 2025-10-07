package httpapi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffetls"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// HTTPServer provides an mTLS HTTP server that authenticates clients using X.509 SVIDs.
type HTTPServer struct {
	server     *http.Server
	x509Source *workloadapi.X509Source
	authorizer tlsconfig.Authorizer
	mux        *http.ServeMux
}

// NewHTTPServer creates an mTLS HTTP server that authenticates clients.
// The authorizer parameter is from go-spiffe and performs identity verification only.
func NewHTTPServer(
	ctx context.Context,
	addr string,
	socketPath string,
	authorizer tlsconfig.Authorizer, // Use go-spiffe authorizers only
) (*HTTPServer, error) {
	// Validate inputs
	if addr == "" {
		return nil, fmt.Errorf("address is required")
	}
	if socketPath == "" {
		return nil, fmt.Errorf("socket path is required")
	}
	if authorizer == nil {
		return nil, fmt.Errorf("authorizer is required")
	}

	// Create X.509 source from SPIRE Workload API
	// This handles automatic SVID rotation
	x509Source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(
			workloadapi.WithAddr(socketPath),
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
		x509Source, // SVID source (server certificate)
		x509Source, // Bundle source (trusted CAs)
		authorizer, // Identity verification (go-spiffe only)
	)

	mux := http.NewServeMux()

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return &HTTPServer{
		server:     server,
		x509Source: x509Source,
		authorizer: authorizer,
		mux:        mux,
	}, nil
}

// Start starts the mTLS HTTP server.
func (s *HTTPServer) Start(ctx context.Context) error {
	// Start server with mTLS
	// Uses certificates from TLSConfig (GetCertificate callback)
	errChan := make(chan error, 1)
	go func() {
		// Empty strings for certFile and keyFile because TLSConfig provides them
		if err := s.server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("httpapi server error: %v", err)
			errChan <- err
		}
	}()

	// Wait briefly to catch immediate startup errors
	select {
	case err := <-errChan:
		return err
	case <-time.After(100 * time.Millisecond):
		log.Printf("httpapi server listening on %s with mTLS", s.server.Addr)
		return nil
	}
}

// Stop gracefully shuts down the server and releases resources.
func (s *HTTPServer) Stop(ctx context.Context) error {
	// Close X509Source (stops SVID fetching and rotation)
	if s.x509Source != nil {
		if err := s.x509Source.Close(); err != nil {
			log.Printf("error closing X509Source: %v", err)
		}
	}

	// Shutdown HTTP server
	if s.server != nil {
		if err := s.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
	}

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
