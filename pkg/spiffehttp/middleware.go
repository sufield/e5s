package spiffehttp

import "net/http"

// SPIFFEAuthMiddleware is an HTTP middleware that extracts and verifies the
// authenticated peer's SPIFFE identity from the mTLS connection, then attaches
// it to the request context for downstream handlers to use.
//
// This middleware:
//   - Extracts peer identity once per request (avoiding repeated TLS state parsing)
//   - Stores identity in request context using WithPeer()
//   - Returns 401 Unauthorized if no valid SPIFFE identity is present
//   - Only allows requests through if mTLS verification succeeded
//
// SECURITY: This middleware assumes the server is using a TLS config returned
// by spiffehttp.NewServerTLSConfig(). If the server is not enforcing mTLS with
// SPIFFE verification, this middleware will reject all requests (because no
// valid peer identity will be found).
//
// Usage with standard net/http:
//
//	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	    peer, _ := spiffehttp.PeerFromContext(r.Context())
//	    fmt.Fprintf(w, "Hello %s\n", peer.ID.String())
//	})
//
//	protectedHandler := spiffehttp.SPIFFEAuthMiddleware(handler)
//
//	server := &http.Server{
//	    Addr:      ":8443",
//	    Handler:   protectedHandler,
//	    TLSConfig: tlsCfg, // from spiffehttp.NewServerTLSConfig
//	}
//	server.ListenAndServeTLS("", "")
//
// Usage with chi router:
//
//	r := chi.NewRouter()
//	r.Use(spiffehttp.SPIFFEAuthMiddleware)
//	r.Get("/api/resource", func(w http.ResponseWriter, r *http.Request) {
//	    peer, _ := spiffehttp.PeerFromContext(r.Context())
//	    // ... use peer.ID for authorization decisions
//	})
//
// For more fine-grained control (e.g., custom error responses or per-route
// policies), you can build custom middleware using PeerFromRequest and
// WithPeer directly.
func SPIFFEAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, ok := PeerFromRequest(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := WithPeer(r.Context(), peer)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
