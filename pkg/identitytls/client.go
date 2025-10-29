package identitytls

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

// ClientConfig configures an mTLS client's server verification policy.
//
// Exactly one of ExpectedServerID or ExpectedServerTrustDomain must be set.
type ClientConfig struct {
	// ExpectedServerID is the exact SPIFFE ID the client expects from the server.
	// Example: "spiffe://example.org/api"
	//
	// Mutually exclusive with ExpectedServerTrustDomain.
	ExpectedServerID string

	// ExpectedServerTrustDomain accepts any server in the specified trust domain.
	// Example: "example.org"
	//
	// Mutually exclusive with ExpectedServerID.
	ExpectedServerTrustDomain string
}

// NewClientTLSConfig creates a TLS configuration for an mTLS client.
//
// The returned *tls.Config is safe to use directly in net/http Transport.
//
// The returned *tls.Config:
//   - Presents the client's certificate for authentication
//   - Enforces TLS 1.3 minimum
//   - Dynamically fetches the client's certificate from the source at dial time
//   - Verifies server identity according to cfg policy
//   - Supports automatic certificate rotation via the source
//
// Server verification policy (exactly one must be configured):
//  1. cfg.ExpectedServerID: require exact SPIFFE ID match
//  2. cfg.ExpectedServerTrustDomain: require server's SPIFFE ID to be in that trust domain
//
// The svidSource and bundleSource parameters must not be nil. They provide:
//   - Client's identity certificate (fetched dynamically at dial time)
//   - Trust bundle for verifying server certificates
//
// These sources' lifetime must exceed the TLS config's lifetime. Close the sources
// only after all clients using this config have closed their connections.
//
// Example:
//
//	source, _ := spire.NewSource(ctx, spire.Config{})
//	defer source.Close()
//	x509Source := source.X509Source()
//
//	tlsCfg, _ := identitytls.NewClientTLSConfig(ctx, x509Source, x509Source, identitytls.ClientConfig{
//	    ExpectedServerID: "spiffe://example.org/api",
//	})
//
//	transport := &http.Transport{
//	    TLSClientConfig: tlsCfg,
//	}
//	client := &http.Client{Transport: transport}
//	resp, _ := client.Get("https://localhost:8443/api")
//
// Context lifetime:
// ctx is ONLY used for initial validation. After NewClientTLSConfig returns,
// the returned *tls.Config will continue to call the sources for new connections
// even if ctx is canceled. To actually shut down identity for this client, you
// must close the sources. Canceling ctx is NOT enough.
//
// Returns error if:
//   - svidSource or bundleSource is nil
//   - Neither ExpectedServerID nor ExpectedServerTrustDomain is set
//   - Both ExpectedServerID and ExpectedServerTrustDomain are set
//   - ExpectedServerID is invalid SPIFFE ID format
func NewClientTLSConfig(ctx context.Context, svidSource x509svid.Source, bundleSource x509bundle.Source, cfg ClientConfig) (*tls.Config, error) {
	if err := validateClientInputs(ctx, svidSource, bundleSource, cfg); err != nil {
		return nil, err
	}

	authorizer, err := buildClientAuthorizer(cfg)
	if err != nil {
		return nil, err
	}

	tlsCfg := tlsconfig.MTLSClientConfig(svidSource, bundleSource, authorizer)
	tlsCfg.MinVersion = tls.VersionTLS13

	return tlsCfg, nil
}

func validateClientInputs(ctx context.Context, svidSource x509svid.Source, bundleSource x509bundle.Source, cfg ClientConfig) error {
	switch {
	case ctx == nil:
		return errors.New("context cannot be nil")
	case svidSource == nil:
		return errors.New("svidSource cannot be nil")
	case bundleSource == nil:
		return errors.New("bundleSource cannot be nil")
	case cfg.ExpectedServerID == "" && cfg.ExpectedServerTrustDomain == "":
		return errors.New("either ExpectedServerID or ExpectedServerTrustDomain must be set")
	case cfg.ExpectedServerID != "" && cfg.ExpectedServerTrustDomain != "":
		return errors.New("ExpectedServerID and ExpectedServerTrustDomain are mutually exclusive")
	}

	// Validate SPIFFE ID format
	if cfg.ExpectedServerID != "" {
		if _, err := spiffeid.FromString(cfg.ExpectedServerID); err != nil {
			return fmt.Errorf("invalid ExpectedServerID: %w", err)
		}
	}

	// Validate trust domain format
	if cfg.ExpectedServerTrustDomain != "" {
		if _, err := spiffeid.TrustDomainFromString(cfg.ExpectedServerTrustDomain); err != nil {
			return fmt.Errorf("invalid ExpectedServerTrustDomain: %w", err)
		}
	}

	return nil
}

func buildClientAuthorizer(cfg ClientConfig) (tlsconfig.Authorizer, error) {
	switch {
	case cfg.ExpectedServerID != "":
		// Policy 1: Exact SPIFFE ID match
		id, err := spiffeid.FromString(cfg.ExpectedServerID)
		if err != nil {
			return nil, fmt.Errorf("invalid ExpectedServerID: %w", err)
		}
		return tlsconfig.AuthorizeID(id), nil

	case cfg.ExpectedServerTrustDomain != "":
		// Policy 2: Trust domain match
		td, err := spiffeid.TrustDomainFromString(cfg.ExpectedServerTrustDomain)
		if err != nil {
			return nil, fmt.Errorf("invalid ExpectedServerTrustDomain: %w", err)
		}
		return tlsconfig.AuthorizeMemberOf(td), nil

	default:
		// Should never reach here due to config validation
		return nil, errors.New("no server verification policy configured (internal misconfiguration)")
	}
}

