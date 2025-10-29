package identitytls

import (
	"net/http"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls"
)

// PeerInfo represents the authenticated identity extracted from an mTLS connection.
// This contains only safe, non-sensitive identity information suitable for
// authorization decisions.
//
// Security: PeerInfo does NOT contain private keys or raw certificate data.
// It only exposes the verified SPIFFE ID and metadata.
type PeerInfo struct {
	// ID is the verified SPIFFE ID from the peer's certificate.
	// This provides strongly-typed access to both the full SPIFFE ID
	// and its components (trust domain, path).
	//
	// Access methods:
	//   - ID.String() returns the full SPIFFE ID (e.g., "spiffe://example.org/service")
	//   - ID.TrustDomain().Name() returns the trust domain (e.g., "example.org")
	//   - ID.Path() returns the workload path (e.g., "/service")
	//   - ID.MemberOf(td) checks trust domain membership
	ID spiffeid.ID

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
//	    // Use peer.ID for authorization decisions
//	    log.Printf("request from %s", peer.ID.String())
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
		ID:        peerID,
		ExpiresAt: r.TLS.PeerCertificates[0].NotAfter,
	}, true
}

