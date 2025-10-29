package identitytls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
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
// The source parameter must not be nil. It provides:
//   - Client's identity certificate (fetched dynamically at dial time)
//   - Trust bundle for verifying server certificates
//
// The source's lifetime must exceed the TLS config's lifetime. Close the source
// only after all clients using this config have closed their connections.
//
// Example:
//
//	source, _ := spire.NewSource(ctx, spire.Config{})
//	defer source.Close()
//
//	tlsCfg, _ := identitytls.NewClientTLSConfig(ctx, source, identitytls.ClientConfig{
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
// ctx is ONLY used for initial validation (first cert fetch and first trust
// bundle fetch). After NewClientTLSConfig returns, the returned *tls.Config
// will continue to call source.GetTLSCertificate()/GetRootCAs() for new
// connections even if ctx is canceled. To actually shut down identity for
// this client, you must call source.Close(). Canceling ctx is NOT enough.
//
// Returns error if:
//   - source is nil
//   - Neither ExpectedServerID nor ExpectedServerTrustDomain is set
//   - Both ExpectedServerID and ExpectedServerTrustDomain are set
//   - ExpectedServerID is invalid SPIFFE ID format
//   - Initial certificate or trust bundle fetch fails
func NewClientTLSConfig(ctx context.Context, source CertSource, cfg ClientConfig) (*tls.Config, error) {
	if ctx == nil {
		return nil, errors.New("context cannot be nil")
	}

	if source == nil {
		return nil, errors.New("source cannot be nil")
	}

	// Validate config
	if cfg.ExpectedServerID == "" && cfg.ExpectedServerTrustDomain == "" {
		return nil, errors.New("either ExpectedServerID or ExpectedServerTrustDomain must be set")
	}

	if cfg.ExpectedServerID != "" && cfg.ExpectedServerTrustDomain != "" {
		return nil, errors.New("ExpectedServerID and ExpectedServerTrustDomain are mutually exclusive")
	}

	// Validate SPIFFE ID if provided
	if cfg.ExpectedServerID != "" {
		if _, err := spiffeid.FromString(cfg.ExpectedServerID); err != nil {
			return nil, fmt.Errorf("invalid ExpectedServerID: %w", err)
		}
	}

	// Basic sanity check for trust domain input.
	// We don't try to fully parse DNS-like labels here, but we do reject obviously bad forms
	// that will never match a SPIFFE ID at runtime.
	if cfg.ExpectedServerTrustDomain != "" {
		if strings.ContainsAny(cfg.ExpectedServerTrustDomain, "/?#") {
			return nil, fmt.Errorf("invalid ExpectedServerTrustDomain %q: trust domain must not contain '/', '?', or '#'", cfg.ExpectedServerTrustDomain)
		}
	}

	// Fetch initial certificate to validate source works
	_, err := source.GetTLSCertificate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get client certificate: %w", err)
	}

	// Fetch initial trust bundle to validate source works
	rootCAs, err := source.GetRootCAs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get trust bundle: %w", err)
	}

	// Build TLS config
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS13,

		// RootCAs is populated here mostly for operator visibility and debugging.
		// IMPORTANT:
		//   • Go's built-in verifier is DISABLED because InsecureSkipVerify=true.
		//   • We DO NOT rely on this RootCAs for security decisions.
		//   • Real verification happens in VerifyConnection, which:
		//       – pulls a FRESH trust bundle from source on every handshake
		//       – re-verifies the presented chain against that fresh bundle
		//       – enforces SPIFFE ID / trust-domain policy
		//
		// Do not remove VerifyConnection assuming RootCAs is "good enough". It is not.
		RootCAs: rootCAs,

		// GetClientCertificate is called during each TLS handshake to fetch the client's certificate.
		// This enables automatic rotation - when the source updates its certificate,
		// new connections will use the fresh cert without restarting the client.
		//
		// Note: CertificateRequestInfo has no Context() method, so we use context.Background().
		// The source must serve from in-memory state (per CertSource contract).
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			cert, err := source.GetTLSCertificate(context.Background())
			if err != nil {
				return nil, fmt.Errorf("failed to get certificate during handshake: %w", err)
			}
			return &cert, nil
		},

		// VerifyConnection is called after standard certificate verification.
		// We use this to:
		//  1. Refresh the trust bundle for rotation support
		//  2. Enforce SPIFFE ID policy (exact match or trust domain match)
		//
		// Note: ConnectionState has no Context() method, so we use context.Background().
		// The source must serve from in-memory state (per CertSource contract).
		VerifyConnection: func(cs tls.ConnectionState) error {
			// Get fresh trust bundle for verification
			// This ensures trust bundle rotation works
			freshRootCAs, err := source.GetRootCAs(context.Background())
			if err != nil {
				return fmt.Errorf("failed to get trust bundle during verification: %w", err)
			}

			// Re-verify server certificate chain with fresh trust bundle
			if len(cs.PeerCertificates) == 0 {
				return errors.New("server presented no certificates")
			}

			serverCert := cs.PeerCertificates[0]

			// Build verification options
			opts := x509.VerifyOptions{
				Roots:         freshRootCAs,
				Intermediates: x509.NewCertPool(),
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}

			// Add intermediate certs to pool
			for i := 1; i < len(cs.PeerCertificates); i++ {
				opts.Intermediates.AddCert(cs.PeerCertificates[i])
			}

			// Re-verify certificate chain
			// Verify returns error if no valid chains exist, so we don't need to check len(chains)
			_, err = serverCert.Verify(opts)
			if err != nil {
				return fmt.Errorf("server certificate verification failed: %w", err)
			}

			// Extract server's SPIFFE ID
			serverID, err := extractSPIFFEID(serverCert)
			if err != nil {
				// Use "authorization failed" prefix so callers can distinguish auth from TLS errors
				return fmt.Errorf("authorization failed: server certificate has no valid SPIFFE ID: %w", err)
			}

			// Apply SPIFFE ID policy
			if err := verifyServerIdentity(serverID, cfg); err != nil {
				return err // Already prefixed with "authorization failed:"
			}

			return nil
		},

		// We intentionally set InsecureSkipVerify=true because we are doing our own
		// verification in VerifyConnection (see above). The standard verifier:
		//   • doesn't know SPIFFE ID semantics
		//   • won't enforce trust-domain policy
		//   • won't pick up rotated bundles mid-process
		//
		// SECURITY NOTE:
		// This is NOT skipping verification. It is replacing it with stricter,
		// SPIFFE-aware verification above. Do not flip this to false and then
		// delete VerifyConnection "to simplify the code".
		InsecureSkipVerify: true,
	}

	return tlsCfg, nil
}

// verifyServerIdentity checks if a server's SPIFFE ID is allowed according to policy.
//
// Policy:
//  1. If cfg.ExpectedServerID is set: require exact match
//  2. If cfg.ExpectedServerTrustDomain is set: require trust domain match
//
// Uses go-spiffe SDK Matcher API for robust matching.
//
// Returns nil if allowed, error describing why denied otherwise.
func verifyServerIdentity(serverID spiffeid.ID, cfg ClientConfig) error {
	// Build matcher based on policy
	var matcher spiffeid.Matcher

	// Policy 1: Exact SPIFFE ID match
	if cfg.ExpectedServerID != "" {
		expectedID, err := spiffeid.FromString(cfg.ExpectedServerID)
		if err != nil {
			// Should never happen - validated in NewClientTLSConfig
			return fmt.Errorf("authorization failed: invalid ExpectedServerID config: %w", err)
		}
		matcher = spiffeid.MatchID(expectedID)
	} else if cfg.ExpectedServerTrustDomain != "" {
		// Policy 2: Trust domain match
		td, err := spiffeid.TrustDomainFromString(cfg.ExpectedServerTrustDomain)
		if err != nil {
			// Should never happen - validated in NewClientTLSConfig
			return fmt.Errorf("authorization failed: invalid ExpectedServerTrustDomain config: %w", err)
		}
		matcher = spiffeid.MatchMemberOf(td)
	} else {
		// Should never reach here due to config validation
		return errors.New("no server verification policy configured (internal misconfiguration)")
	}

	// Apply matcher
	if err := matcher(serverID); err != nil {
		return fmt.Errorf("authorization failed: %w", err)
	}

	return nil
}
