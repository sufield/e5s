package identitytls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
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
//   - Requires client certificates (mutual TLS) and verifies them in VerifyPeerCertificate
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
// The source parameter must not be nil. It provides:
//   - Server's identity certificate (fetched dynamically at handshake time)
//   - Trust bundle for verifying client certificates
//
// The source's lifetime must exceed the TLS config's lifetime. Close the source
// only after all servers using this config have stopped.
//
// Example:
//
//	source, _ := spire.NewSource(ctx, spire.Config{})
//	defer source.Close()
//
//	tlsCfg, _ := identitytls.NewServerTLSConfig(ctx, source, identitytls.ServerConfig{
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
// ctx is ONLY used for initial validation (fetch server cert, extract server
// trust domain, fetch initial trust bundle). After NewServerTLSConfig returns,
// the server's tls.Config will continue to call source.GetTLSCertificate() /
// GetRootCAs() during new handshakes even if ctx is canceled. To stop serving
// new verified connections, you must stop the server AND call source.Close().
// Canceling ctx alone does NOT tear down mTLS identity.
//
// Returns error if:
//   - source is nil
//   - Both AllowedClientID and AllowedClientTrustDomain are set
//   - Initial certificate or trust bundle fetch fails
//   - Server certificate has no SPIFFE ID
func NewServerTLSConfig(ctx context.Context, source CertSource, cfg ServerConfig) (*tls.Config, error) {
	if ctx == nil {
		return nil, errors.New("context cannot be nil")
	}

	if source == nil {
		return nil, errors.New("source cannot be nil")
	}

	// Validate config
	if cfg.AllowedClientID != "" && cfg.AllowedClientTrustDomain != "" {
		return nil, errors.New("AllowedClientID and AllowedClientTrustDomain are mutually exclusive")
	}

	// Validate SPIFFE IDs if provided
	if cfg.AllowedClientID != "" {
		if err := ValidateSPIFFEID(cfg.AllowedClientID); err != nil {
			return nil, fmt.Errorf("invalid AllowedClientID: %w", err)
		}
	}

	// Basic sanity check for trust domain input.
	// We don't try to fully parse DNS-like labels here, but we do reject obviously bad forms
	// that will never match a SPIFFE ID at runtime.
	if cfg.AllowedClientTrustDomain != "" {
		if strings.ContainsAny(cfg.AllowedClientTrustDomain, "/?#") {
			return nil, fmt.Errorf("invalid AllowedClientTrustDomain %q: trust domain must not contain '/', '?', or '#'", cfg.AllowedClientTrustDomain)
		}
	}

	// Fetch initial certificate to validate source works and extract server trust domain
	serverCert, err := source.GetTLSCertificate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get server certificate: %w", err)
	}

	// Guard against nil Leaf
	if serverCert.Leaf == nil {
		return nil, errors.New("cert source returned certificate with no parsed Leaf; source must populate Leaf field")
	}

	// Extract server's SPIFFE ID and trust domain
	_, serverTrustDomain, err := extractSPIFFEID(serverCert.Leaf)
	if err != nil {
		return nil, fmt.Errorf("failed to extract server SPIFFE ID: %w", err)
	}

	// Fetch initial trust bundle to validate source works
	_, err = source.GetRootCAs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get trust bundle: %w", err)
	}

	// Build TLS config
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS13,

		// We intentionally use tls.RequireAnyClientCert instead of tls.RequireAndVerifyClientCert.
		//
		// RATIONALE:
		//   • Go's built-in TLS verifier doesn't understand SPIFFE IDs or trust-domain policy.
		//   • It also won't hot-reload trust bundles per-handshake.
		//   • We do full verification ourselves in VerifyPeerCertificate, including:
		//       – verifying the presented chain against a FRESH trust bundle from source
		//       – enforcing SPIFFE ID / trust-domain policy
		//
		// SECURITY NOTE:
		// Do NOT "simplify" this by switching to RequireAndVerifyClientCert and deleting
		// VerifyPeerCertificate. That would silently drop SPIFFE-based authorization.
		ClientAuth: tls.RequireAnyClientCert,

		// GetCertificate is called during each TLS handshake to fetch the server's certificate.
		// This enables automatic rotation - when the source updates its certificate,
		// new handshakes will use the fresh cert without restarting the server.
		//
		// Note: ClientHelloInfo has no Context() method, so we use context.Background().
		// The source must serve from in-memory state (per CertSource contract).
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, err := source.GetTLSCertificate(context.Background())
			if err != nil {
				return nil, fmt.Errorf("failed to get certificate during handshake: %w", err)
			}
			return &cert, nil
		},

		// VerifyPeerCertificate is called after the handshake completes.
		// We use this to:
		//  1. Manually verify the client certificate chain against a fresh trust bundle
		//  2. Enforce SPIFFE ID policy (exact match or trust domain match)
		//
		// We ignore verifiedChains because we're doing full verification ourselves
		// using a fresh CA bundle from source. This guarantees bundle rotation and
		// consistent SPIFFE policy enforcement per handshake.
		//
		// This approach ensures:
		//  - Trust bundle rotation works (we fetch fresh CAs on every handshake)
		//  - Policy enforcement is consistent (no config drift bugs)
		//  - We control the entire verification flow
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("no client certificate presented")
			}

			// Parse leaf certificate
			leaf, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return fmt.Errorf("failed to parse client leaf cert: %w", err)
			}

			// Parse intermediate certificates
			intermediates := x509.NewCertPool()
			for _, rawCert := range rawCerts[1:] {
				cert, err := x509.ParseCertificate(rawCert)
				if err != nil {
					return fmt.Errorf("failed to parse client intermediate cert: %w", err)
				}
				intermediates.AddCert(cert)
			}

			// Fetch fresh trust bundle (must be fast; CertSource is required to serve
			// this from in-memory state, not do network I/O here).
			// This ensures trust bundle rotation works.
			roots, err := source.GetRootCAs(context.Background())
			if err != nil {
				return fmt.Errorf("failed to get trust bundle during verification: %w", err)
			}

			// Verify certificate chain ourselves
			_, err = leaf.Verify(x509.VerifyOptions{
				Roots:         roots,
				Intermediates: intermediates,
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			})
			if err != nil {
				return fmt.Errorf("client certificate verification failed: %w", err)
			}

			// Extract client's SPIFFE ID
			clientSPIFFEID, clientTrustDomain, err := extractSPIFFEID(leaf)
			if err != nil {
				// Use "authorization failed" prefix so callers can distinguish auth from TLS errors
				return fmt.Errorf("authorization failed: client certificate has no valid SPIFFE ID: %w", err)
			}

			// Apply SPIFFE ID policy
			if err := verifyClientIdentity(clientSPIFFEID, clientTrustDomain, serverTrustDomain, cfg); err != nil {
				return err // Already prefixed with "authorization failed:"
			}

			return nil
		},
	}

	return tlsCfg, nil
}

// verifyClientIdentity checks if a client's SPIFFE ID is allowed according to policy.
//
// Policy (evaluated in order):
//  1. If cfg.AllowedClientID is set: require exact match
//  2. If cfg.AllowedClientTrustDomain is set: require trust domain match
//  3. Otherwise: require same trust domain as server
//
// Returns nil if allowed, error describing why denied otherwise.
func verifyClientIdentity(clientSPIFFEID, clientTrustDomain, serverTrustDomain string, cfg ServerConfig) error {
	// Policy 1: Exact SPIFFE ID match
	if cfg.AllowedClientID != "" {
		if clientSPIFFEID == cfg.AllowedClientID {
			return nil // Allowed
		}
		return fmt.Errorf("authorization failed: client SPIFFE ID %q does not match allowed ID %q", clientSPIFFEID, cfg.AllowedClientID)
	}

	// Policy 2: Trust domain match (explicit)
	if cfg.AllowedClientTrustDomain != "" {
		if MatchesTrustDomain(clientSPIFFEID, cfg.AllowedClientTrustDomain) {
			return nil // Allowed
		}
		return fmt.Errorf("authorization failed: client trust domain %q does not match allowed trust domain %q", clientTrustDomain, cfg.AllowedClientTrustDomain)
	}

	// Policy 3: Same trust domain as server (default)
	if MatchesTrustDomain(clientSPIFFEID, serverTrustDomain) {
		return nil // Allowed
	}

	return fmt.Errorf("authorization failed: client trust domain %q does not match server trust domain %q", clientTrustDomain, serverTrustDomain)
}
