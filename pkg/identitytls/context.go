package identitytls

import "context"

// peerInfoKey is the context key for storing PeerInfo.
// Using an unexported type prevents collisions with other packages.
type peerInfoKey struct{}

// peerInfoCtxKey is the singleton key instance for context operations.
// Reusing the same key avoids allocating a new literal on each call.
var peerInfoCtxKey = peerInfoKey{}

// WithPeerInfo attaches PeerInfo to a context.
//
// This is typically used in middleware to store the authenticated identity
// after extracting it from the TLS connection. Handlers downstream can then
// retrieve it using PeerInfoFromContext.
//
// PeerInfo is stored in context by value (not by pointer) to avoid accidental
// mutation after authorization.
//
// Example middleware pattern:
//
//	func authMiddleware(next http.Handler) http.Handler {
//	    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	        peer, ok := identitytls.ExtractPeerInfo(r)
//	        if !ok {
//	            http.Error(w, "unauthorized", http.StatusUnauthorized)
//	            return
//	        }
//	        ctx := identitytls.WithPeerInfo(r.Context(), peer)
//	        next.ServeHTTP(w, r.WithContext(ctx))
//	    })
//	}
//
// Safe to call with zero-value PeerInfo (it will be stored as-is).
func WithPeerInfo(ctx context.Context, peer PeerInfo) context.Context {
	return context.WithValue(ctx, peerInfoCtxKey, peer)
}

// PeerInfoFromContext retrieves PeerInfo from a context.
//
// Returns:
//   - PeerInfo: the authenticated peer identity
//   - bool: true if PeerInfo was found in context
//
// Example handler:
//
//	func myHandler(w http.ResponseWriter, r *http.Request) {
//	    peer, ok := identitytls.PeerInfoFromContext(r.Context())
//	    if !ok {
//	        http.Error(w, "unauthorized", http.StatusUnauthorized)
//	        return
//	    }
//	    // Use peer.ID for authorization
//	    log.Printf("Request from %s", peer.ID.String())
//	}
//
// Returns false if:
//   - Context is nil
//   - PeerInfo was never attached via WithPeerInfo
//   - Context value is not of type PeerInfo (shouldn't happen with unexported key)
func PeerInfoFromContext(ctx context.Context) (PeerInfo, bool) {
	if ctx == nil {
		return PeerInfo{}, false
	}

	peer, ok := ctx.Value(peerInfoCtxKey).(PeerInfo)
	return peer, ok
}
