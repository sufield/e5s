package identitytls

import "context"

// contextKey is the type for context keys.
// Using an unexported type prevents collisions with keys from other packages.
type contextKey int

const (
	// peerKey is the context key for peer information
	peerKey contextKey = iota
)

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
//	        peer, ok := identitytls.ExtractPeerInfo(r)
//	        if !ok {
//	            http.Error(w, "unauthorized", http.StatusUnauthorized)
//	            return
//	        }
//	        ctx := identitytls.WithPeer(r.Context(), peer)
//	        next.ServeHTTP(w, r.WithContext(ctx))
//	    })
//	}
func WithPeer(ctx context.Context, peer PeerInfo) context.Context {
	return context.WithValue(ctx, peerKey, peer)
}

// PeerFromContext retrieves peer information from the context.
//
// This follows the SDK's naming pattern (similar to spiffegrpc.PeerFromContext).
// Returns (PeerInfo{}, false) if the context has no peer info.
//
// Example handler:
//
//	func myHandler(w http.ResponseWriter, r *http.Request) {
//	    peer, ok := identitytls.PeerFromContext(r.Context())
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
func PeerFromContext(ctx context.Context) (PeerInfo, bool) {
	if ctx == nil {
		return PeerInfo{}, false
	}
	peer, ok := ctx.Value(peerKey).(PeerInfo)
	return peer, ok
}
