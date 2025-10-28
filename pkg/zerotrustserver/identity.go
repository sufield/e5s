package zerotrustserver

import (
	"context"

	"github.com/sufield/e5s/internal/ports"
)

// Identity is the authenticated workload identity.
// Re-exported from internal/ports to provide a single public entry point.
//
// This type alias allows users to import only the facade package
// (pkg/zerotrustserver) without directly importing internal packages.
type Identity = ports.Identity

// PeerIdentity retrieves the authenticated identity from the request context.
//
// Returns (identity, true) if the request was authenticated by the mTLS middleware,
// or (zero-value, false) if no identity is present.
//
// This is the primary way application handlers access authenticated identity.
//
// Example:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    id, ok := zerotrustserver.PeerIdentity(r.Context())
//	    if !ok {
//	        http.Error(w, "unauthorized", http.StatusUnauthorized)
//	        return
//	    }
//	    fmt.Fprintf(w, "Hello, %s\n", id.SPIFFEID)
//	}
func PeerIdentity(ctx context.Context) (Identity, bool) {
	return ports.PeerIdentity(ctx)
}
