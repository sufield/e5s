package spiffehttp

import (
	"context"
	"net/http"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls"
)

// Peer represents the authenticated identity extracted from an mTLS connection.
//
// This contains only safe, non-sensitive identity information suitable for
// authorization decisions.
//
// Security: Peer does NOT contain private keys or raw certificate data.
// It only exposes the verified SPIFFE ID and metadata.
type Peer struct {
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

// PeerFromRequest extracts the authenticated caller's identity from an mTLS HTTP request.
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
// PeerFromRequest DOES NOT authenticate the caller by itself. It only reads
// whatever certificate Go attached to r.TLS.
//
// You MUST only treat the returned Peer as trusted if BOTH are true:
//  1. The server handling this request was started with a tls.Config returned
//     by spiffehttp.NewServerTLSConfig(...) or SDK's tlsconfig.MTLSServerConfig.
//  2. The TLS handshake for this request succeeded, which means SPIFFE ID
//     verification ran and accepted the client's identity.
//
// If the server was not using SDK mTLS verification (for example, someone used
// a plain net/http.Server with default TLS settings or with ClientAuth disabled),
// then PeerFromRequest can return a SPIFFE ID taken from an UNVERIFIED certificate.
// That is attacker-controlled data.
//
// In short: only call this behind SDK mTLS server config, after successful
// SPIFFE verification. Never call this on traffic from an arbitrary proxy or
// from a server that didn't enforce mTLS.
//
// Usage in a handler:
//
//	func myHandler(w http.ResponseWriter, r *http.Request) {
//	    peer, ok := spiffehttp.PeerFromRequest(r)
//	    if !ok {
//	        http.Error(w, "unauthorized", http.StatusUnauthorized)
//	        return
//	    }
//	    // Use peer.ID for authorization decisions
//	    log.Printf("request from %s", peer.ID.String())
//	}
//
// Returns:
//   - Peer with verified identity information
//   - true if identity was successfully extracted
//   - false if no TLS connection, no peer cert, or SPIFFE ID parsing failed
//
// This does NOT verify that the peer is authorized for a specific resource.
// It only extracts the authenticated identity. Authorization logic belongs
// in your handlers.
func PeerFromRequest(r *http.Request) (Peer, bool) {
	// Check for valid request and TLS connection
	if r == nil || r.TLS == nil {
		return Peer{}, false
	}

	// Use SDK to extract peer SPIFFE ID from TLS connection state.
	// PeerIDFromConnectionState will fail if there are no peer certs or the ID
	// can't be parsed as a SPIFFE ID.
	id, err := spiffetls.PeerIDFromConnectionState(*r.TLS)
	if err != nil {
		return Peer{}, false
	}

	// Defensive: PeerIDFromConnectionState already guarantees at least one
	// peer certificate, but check to avoid panics if assumptions change.
	var expiresAt time.Time
	if len(r.TLS.PeerCertificates) > 0 && r.TLS.PeerCertificates[0] != nil {
		expiresAt = r.TLS.PeerCertificates[0].NotAfter
	}

	return Peer{ID: id, ExpiresAt: expiresAt}, true
}

// context key type (unexported) to avoid collisions with keys from other packages.
type peerCtxKey struct{}

var peerKey = peerCtxKey{}

// WithPeer attaches peer information to the context.
//
// This follows the SDK's naming pattern (similar to spiffegrpc.WithPeer).
// Typically used in middleware to store the authenticated identity after
// extracting it from the TLS connection.
//
// Example middleware:
//
//	func authMiddleware(next http.Handler) http.Handler {
//	    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	        peer, ok := spiffehttp.PeerFromRequest(r)
//	        if !ok {
//	            http.Error(w, "unauthorized", http.StatusUnauthorized)
//	            return
//	        }
//	        ctx := spiffehttp.WithPeer(r.Context(), peer)
//	        next.ServeHTTP(w, r.WithContext(ctx))
//	    })
//	}
func WithPeer(ctx context.Context, p Peer) context.Context {
	return context.WithValue(ctx, peerKey, p)
}

// PeerFromContext retrieves peer information from the context.
//
// This follows the SDK's naming pattern (similar to spiffegrpc.PeerFromContext).
// Returns (Peer{}, false) if the context has no peer info.
//
// Example handler:
//
//	func myHandler(w http.ResponseWriter, r *http.Request) {
//	    peer, ok := spiffehttp.PeerFromContext(r.Context())
//	    if !ok {
//	        http.Error(w, "unauthorized", http.StatusUnauthorized)
//	        return
//	    }
//
//	    // Use strongly-typed spiffeid.ID methods
//	    if !peer.ID.MemberOf(myTrustDomain) {
//	        http.Error(w, "forbidden", http.StatusForbidden)
//	        return
//	    }
//
//	    log.Printf("Request from %s in %s",
//	        peer.ID.Path(), peer.ID.TrustDomain().Name())
//	}
func PeerFromContext(ctx context.Context) (Peer, bool) {
	if ctx == nil {
		return Peer{}, false
	}
	p, ok := ctx.Value(peerKey).(Peer)
	return p, ok
}
