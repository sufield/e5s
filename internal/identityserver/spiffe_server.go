package identityserver

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// spiffeServer is the concrete implementation of the Server interface using go-spiffe SDK.
// It completely encapsulates all SPIFFE/SPIRE implementation details.
type spiffeServer struct {
	config     Config
	x509Source *workloadapi.X509Source
	httpServer *http.Server
	mux        *http.ServeMux
}

// New creates a new identity server using SPIFFE/SPIRE for mTLS authentication.
// The server fetches X.509 SVIDs from the SPIRE agent and uses them for mTLS.
// Client authentication is configured based on the Config.SPIFFE settings.
func New(ctx context.Context, cfg Config) (Server, error) {
	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create X.509 source from SPIRE Workload API
	// This handles automatic SVID fetching and rotation
	x509Source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(
			workloadapi.WithAddr(cfg.WorkloadAPI.SocketPath),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create X509Source: %w", err)
	}

	// Create authorizer based on configuration
	authorizer, err := createAuthorizer(cfg.SPIFFE)
	if err != nil {
		x509Source.Close()
		return nil, fmt.Errorf("failed to create authorizer: %w", err)
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

	// Create HTTP server mux
	mux := http.NewServeMux()

	// Create HTTP server
	httpServer := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           mux,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	return &spiffeServer{
		config:     cfg,
		x509Source: x509Source,
		httpServer: httpServer,
		mux:        mux,
	}, nil
}

// Handle registers an HTTP handler for the given pattern.
func (s *spiffeServer) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// Start starts the mTLS HTTP server.
func (s *spiffeServer) Start(ctx context.Context) error {
	// Start server with mTLS
	// Uses certificates from TLSConfig (GetCertificate callback from X509Source)
	go func() {
		// Empty strings for certFile and keyFile because TLSConfig provides them
		if err := s.httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("identity server error: %v", err)
		}
	}()

	log.Printf("identity server listening on %s with mTLS", s.config.HTTP.Address)
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *spiffeServer) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown http server: %w", err)
		}
	}
	return nil
}

// Close immediately closes all connections and releases resources.
func (s *spiffeServer) Close() error {
	// Close HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Close(); err != nil {
			log.Printf("error closing http server: %v", err)
		}
	}

	// Close X509Source (stops SVID fetching and rotation)
	if s.x509Source != nil {
		if err := s.x509Source.Close(); err != nil {
			return fmt.Errorf("failed to close X509Source: %w", err)
		}
	}

	return nil
}

// validateConfig validates the server configuration.
func validateConfig(cfg Config) error {
	if cfg.WorkloadAPI.SocketPath == "" {
		return fmt.Errorf("WorkloadAPI.SocketPath is required")
	}

	if cfg.HTTP.Address == "" {
		return fmt.Errorf("HTTP.Address is required")
	}

	if cfg.HTTP.ReadHeaderTimeout == 0 {
		return fmt.Errorf("HTTP.ReadHeaderTimeout must be > 0")
	}

	return nil
}

// createAuthorizer creates a go-spiffe authorizer based on configuration.
// This implements authentication only - no custom authorization.
func createAuthorizer(cfg SPIFFEConfig) (tlsconfig.Authorizer, error) {
	// Case 1: Specific SPIFFE ID required
	if cfg.AllowedClientID != "" {
		clientID, err := spiffeid.FromString(cfg.AllowedClientID)
		if err != nil {
			return nil, fmt.Errorf("invalid AllowedClientID: %w", err)
		}
		return tlsconfig.AuthorizeID(clientID), nil
	}

	// Case 2: Specific trust domain required
	if cfg.AllowedTrustDomain != "" {
		trustDomain, err := spiffeid.TrustDomainFromString(cfg.AllowedTrustDomain)
		if err != nil {
			return nil, fmt.Errorf("invalid AllowedTrustDomain: %w", err)
		}
		return tlsconfig.AuthorizeMemberOf(trustDomain), nil
	}

	// Case 3: Any authenticated client (same trust domain as server)
	// This is secure because mTLS ensures client has valid SVID from SPIRE
	return tlsconfig.AuthorizeAny(), nil
}
