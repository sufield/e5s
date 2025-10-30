package spiffehttp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

// ServerConfig configures an mTLS server's identity verification policy.
//
// The zero value is valid and enforces: any client in the same trust domain
// as the server is allowed.
type ServerConfig struct {
	// AllowedClientID restricts connections to clients with this exact SPIFFE ID.
	// Example: "spiffe://example.org/client"
	//
	// Mutually exclusive with AllowedClientTrustDomain.
	// If both are empty, any client in the server's trust domain is allowed.
	AllowedClientID string

	// AllowedClientTrustDomain allows any client in the specified trust domain.
	// Example: "example.org"
	//
	// Mutually exclusive with AllowedClientID.
	// If both are empty, any client in the server's trust domain is allowed.
	AllowedClientTrustDomain string
}

// NewServerTLSConfig creates a TLS configuration for an mTLS server.
//
// The returned *tls.Config:
//   - Requires client certificates (mutual TLS) and verifies them
//   - Enforces TLS 1.3 minimum
//   - Dynamically fetches the server's certificate from the source at handshake time
//   - Verifies client identity according to cfg policy
//   - Supports automatic certificate rotation via the source
//
// The returned *tls.Config is safe to use directly in net/http Server.
//
// Client verification policy (evaluated in order):
//  1. If cfg.AllowedClientID is set: only that exact SPIFFE ID is accepted
//  2. If cfg.AllowedClientTrustDomain is set: any SPIFFE ID in that trust domain is accepted
//  3. If both are empty: any client in the same trust domain as the server is accepted
//
// The svidSource and bundleSource parameters must not be nil. They provide:
//   - Server's identity certificate (fetched dynamically at handshake time)
//   - Trust bundle for verifying client certificates
//
// These sources' lifetime must exceed the TLS config's lifetime. Close the sources
// only after all servers using this config have stopped.
//
// Example:
//
//	source, _ := spire.NewSource(ctx, spire.Config{})
//	defer source.Close()
//	x509Source := source.X509Source()
//
//	tlsCfg, _ := spiffehttp.NewServerTLSConfig(ctx, x509Source, x509Source, spiffehttp.ServerConfig{
//	    AllowedClientTrustDomain: "example.org",
//	})
//
//	server := &http.Server{
//	    Addr:      ":8443",
//	    TLSConfig: tlsCfg,
//	}
//	server.ListenAndServeTLS("", "") // Cert/key come from tlsCfg
//
// Context lifetime:
// ctx is ONLY used for initial validation (fetch server cert to extract server
// trust domain for default policy). After NewServerTLSConfig returns, the
// server's tls.Config will continue to call the sources during new handshakes
// even if ctx is canceled. To stop serving new verified connections, you must
// stop the server AND close the sources. Canceling ctx alone does NOT tear down
// mTLS identity.
//
// Returns error if:
//   - svidSource or bundleSource is nil
//   - Both AllowedClientID and AllowedClientTrustDomain are set
//   - Initial certificate fetch fails
//   - Server certificate has no SPIFFE ID
func NewServerTLSConfig(ctx context.Context, svidSource x509svid.Source, bundleSource x509bundle.Source, cfg ServerConfig) (*tls.Config, error) {
	if err := validateServerInputs(ctx, svidSource, bundleSource, cfg); err != nil {
		return nil, err
	}

	authorizer, err := buildServerAuthorizer(svidSource, cfg)
	if err != nil {
		return nil, err
	}

	tlsCfg := tlsconfig.MTLSServerConfig(svidSource, bundleSource, authorizer)
	tlsCfg.MinVersion = tls.VersionTLS13

	return tlsCfg, nil
}

func validateServerInputs(ctx context.Context, svidSource x509svid.Source, bundleSource x509bundle.Source, cfg ServerConfig) error {
	switch {
	case ctx == nil:
		return errors.New("context cannot be nil")
	case svidSource == nil:
		return errors.New("svidSource cannot be nil")
	case bundleSource == nil:
		return errors.New("bundleSource cannot be nil")
	case cfg.AllowedClientID != "" && cfg.AllowedClientTrustDomain != "":
		return errors.New("AllowedClientID and AllowedClientTrustDomain are mutually exclusive")
	}

	// Validate SPIFFE ID format
	if cfg.AllowedClientID != "" {
		if _, err := spiffeid.FromString(cfg.AllowedClientID); err != nil {
			return fmt.Errorf("invalid AllowedClientID: %w", err)
		}
	}

	// Validate trust domain format
	if cfg.AllowedClientTrustDomain != "" {
		if _, err := spiffeid.TrustDomainFromString(cfg.AllowedClientTrustDomain); err != nil {
			return fmt.Errorf("invalid AllowedClientTrustDomain: %w", err)
		}
	}

	return nil
}

func buildServerAuthorizer(svidSource x509svid.Source, cfg ServerConfig) (tlsconfig.Authorizer, error) {
	switch {
	case cfg.AllowedClientID != "":
		// Policy 1: Exact SPIFFE ID match
		id, err := spiffeid.FromString(cfg.AllowedClientID)
		if err != nil {
			return nil, fmt.Errorf("invalid AllowedClientID: %w", err)
		}
		return tlsconfig.AuthorizeID(id), nil

	case cfg.AllowedClientTrustDomain != "":
		// Policy 2: Trust domain match (explicit)
		td, err := spiffeid.TrustDomainFromString(cfg.AllowedClientTrustDomain)
		if err != nil {
			return nil, fmt.Errorf("invalid AllowedClientTrustDomain: %w", err)
		}
		return tlsconfig.AuthorizeMemberOf(td), nil

	default:
		// Policy 3: Same trust domain as server (default)
		svid, err := svidSource.GetX509SVID()
		if err != nil {
			return nil, fmt.Errorf("failed to get server SVID: %w", err)
		}
		return tlsconfig.AuthorizeMemberOf(svid.ID.TrustDomain()), nil
	}
}
