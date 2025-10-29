package identitytls

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

// PeerInfo represents the authenticated identity extracted from an mTLS connection.
// This contains only safe, non-sensitive identity information suitable for
// authorization decisions.
//
// Security: PeerInfo does NOT contain private keys or raw certificate data.
// It only exposes the verified SPIFFE ID and metadata.
type PeerInfo struct {
	// SPIFFEID is the verified SPIFFE ID from the peer's certificate.
	// Example: "spiffe://example.org/service"
	SPIFFEID string

	// TrustDomain is extracted from the SPIFFE ID.
	// Example: "example.org"
	TrustDomain string

	// ExpiresAt is when the peer's certificate expires.
	// After this time, the peer must re-authenticate with a fresh certificate.
	ExpiresAt time.Time
}

// ExtractPeerInfo extracts the authenticated caller's identity from an mTLS HTTP request.
//
// This function inspects the verified TLS connection state and returns the peer's
// SPIFFE ID and certificate metadata. It works with any HTTP framework (chi, gin,
// net/http, etc.) because it only depends on *http.Request.
//
// Uses the official go-spiffe SDK (spiffetls.PeerIDFromConnectionState) for
// extracting the peer SPIFFE ID.
//
// SECURITY NOTE:
//
// ExtractPeerInfo DOES NOT authenticate the caller by itself. It only reads
// whatever certificate Go attached to r.TLS.
//
// You MUST only treat the returned PeerInfo as trusted if BOTH are true:
//   1. The server handling this request was started with a tls.Config returned
//      by identitytls.NewServerTLSConfig(...).
//   2. The TLS handshake for this request succeeded, which means our
//      VerifyPeerCertificate ran and accepted the client's SPIFFE ID.
//
// If the server was not using identitytls.NewServerTLSConfig (for example,
// someone used a plain net/http.Server with default TLS settings or with
// ClientAuth disabled), then ExtractPeerInfo can return a SPIFFE ID taken
// from an UNVERIFIED certificate. That is attacker-controlled data.
//
// In short: only call this behind our mTLS server config, after successful
// SPIFFE verification. Never call this on traffic from an arbitrary proxy or
// from a server that didn't enforce mTLS.
//
// Usage in a handler:
//
//	func myHandler(w http.ResponseWriter, r *http.Request) {
//	    peer, ok := identitytls.ExtractPeerInfo(r)
//	    if !ok {
//	        http.Error(w, "unauthorized", http.StatusUnauthorized)
//	        return
//	    }
//	    // Use peer.SPIFFEID for authorization decisions
//	    log.Printf("request from %s", peer.SPIFFEID)
//	}
//
// Returns:
//   - PeerInfo with verified identity information
//   - true if identity was successfully extracted
//   - false if no TLS connection, no peer cert, or SPIFFE ID parsing failed
//
// This does NOT verify that the peer is authorized for a specific resource.
// It only extracts the authenticated identity. Authorization logic belongs
// in your handlers.
func ExtractPeerInfo(r *http.Request) (PeerInfo, bool) {
	// Check for TLS connection
	if r.TLS == nil {
		return PeerInfo{}, false
	}

	// Use SDK to extract peer SPIFFE ID from TLS connection state
	peerID, err := spiffetls.PeerIDFromConnectionState(*r.TLS)
	if err != nil {
		return PeerInfo{}, false
	}

	return PeerInfo{
		SPIFFEID:    peerID.String(),
		TrustDomain: peerID.TrustDomain().String(),
		ExpiresAt:   r.TLS.PeerCertificates[0].NotAfter,
	}, true
}

// extractSPIFFEID parses the SPIFFE ID from a certificate's URI SAN.
//
// SPIFFE IDs are encoded in X.509 certificates as URI Subject Alternative Names
// with the format: spiffe://<trust-domain>/<path>
//
// This is an internal helper used by server and client TLS verification callbacks.
// It uses the official go-spiffe SDK (x509svid.IDFromCert) for parsing and validation.
//
// Returns the SPIFFE ID as a strongly-typed spiffeid.ID, which provides methods for:
//   - id.String() - full SPIFFE ID as string
//   - id.TrustDomain() - trust domain object
//   - id.Path() - workload path component
//
// Returns error if certificate is nil or has no valid SPIFFE ID.
func extractSPIFFEID(cert *x509.Certificate) (spiffeid.ID, error) {
	if cert == nil {
		return spiffeid.ID{}, errors.New("certificate is nil")
	}

	return x509svid.IDFromCert(cert)
}

// ValidateSPIFFEID validates a SPIFFE ID string format.
//
// Uses the official go-spiffe SDK for validation, ensuring compliance with the SPIFFE spec.
//
// A valid SPIFFE ID must:
//   - Use the "spiffe" URI scheme
//   - Have a non-empty trust domain
//   - Include a workload path (not just "/" or empty)
//   - Not contain query or fragment components
//
// Example valid IDs:
//   - "spiffe://example.org/service"
//   - "spiffe://example.org/ns/production/sa/api"
//
// Example invalid IDs:
//   - "https://example.org/service" (wrong scheme)
//   - "spiffe:///service" (empty trust domain)
//   - "spiffe://example.org/" (missing workload path)
//   - "spiffe://example.org/service?query=param" (has query string)
//
// Returns wrapped spiffeid sentinel errors (use errors.Is for programmatic checks).
func ValidateSPIFFEID(id string) error {
	_, err := spiffeid.FromString(id)
	if err != nil {
		return fmt.Errorf("invalid SPIFFE ID: %w", err)
	}
	return nil
}

// SPIFFEIDTrustDomain extracts the trust domain from a SPIFFE ID.
//
// Uses the official go-spiffe SDK for parsing.
//
// Example:
//
//	SPIFFEIDTrustDomain("spiffe://example.org/service") -> "example.org"
//	SPIFFEIDTrustDomain("spiffe://prod.example.com/api") -> "prod.example.com"
func SPIFFEIDTrustDomain(spiffeID string) (string, error) {
	id, err := spiffeid.FromString(spiffeID)
	if err != nil {
		return "", fmt.Errorf("invalid SPIFFE ID: %w", err)
	}

	return id.TrustDomain().String(), nil
}

// MatchesTrustDomain checks if a SPIFFE ID belongs to a specific trust domain.
//
// Trust domain comparison is case-sensitive per SPIFFE spec (trust domains
// are DNS-like labels, not generic URIs).
//
// Example:
//
//	MatchesTrustDomain("spiffe://example.org/service", "example.org") -> true
//	MatchesTrustDomain("spiffe://Example.org/service", "example.org") -> false
//	MatchesTrustDomain("spiffe://other.org/service", "example.org") -> false
func MatchesTrustDomain(spiffeID, trustDomain string) bool {
	td, err := SPIFFEIDTrustDomain(spiffeID)
	if err != nil {
		return false
	}
	return td == trustDomain
}
