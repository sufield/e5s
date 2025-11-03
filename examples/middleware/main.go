// Package main demonstrates how to build custom HTTP middleware for SPIFFE mTLS.
//
// This is NOT part of the core library - it's a reference implementation showing
// best practices for extracting peer identity and implementing authorization.
//
// Copy and adapt this code for your specific use case.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/sufield/e5s/pkg/spiffehttp"
	"github.com/sufield/e5s/pkg/spire"
)

// authMiddleware is a basic example that extracts peer identity and attaches
// it to the request context.
//
// Customize this for your needs:
//   - Different error responses (403 vs 401, JSON vs plain text)
//   - Custom authorization logic (trust domain checks, path-based rules)
//   - Logging and metrics
//   - Rate limiting per SPIFFE ID
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract peer identity from TLS connection
		peer, ok := spiffehttp.PeerFromRequest(r)
		if !ok {
			http.Error(w, "Unauthorized: no valid SPIFFE identity", http.StatusUnauthorized)
			return
		}

		// Attach to context for downstream handlers
		ctx := spiffehttp.WithPeer(r.Context(), peer)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// trustDomainMiddleware is an example with explicit trust domain authorization.
//
// This shows how to enforce trust domain boundaries - useful when you accept
// mTLS from multiple trust domains but want per-route policies.
func trustDomainMiddleware(allowedTD spiffeid.TrustDomain) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer, ok := spiffehttp.PeerFromRequest(r)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check trust domain
			if !peer.ID.MemberOf(allowedTD) {
				http.Error(w, fmt.Sprintf("Forbidden: must be from trust domain %s", allowedTD.Name()),
					http.StatusForbidden)
				return
			}

			// Log successful authentication
			log.Printf("Authenticated: %s from %s (cert expires: %s)",
				peer.ID.Path(), peer.ID.TrustDomain().Name(), peer.ExpiresAt)

			ctx := spiffehttp.WithPeer(r.Context(), peer)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// certExpiryWarningMiddleware logs warnings for certificates expiring soon.
//
// This is useful for debugging certificate rotation issues.
func certExpiryWarningMiddleware(warningThreshold time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer, ok := spiffehttp.PeerFromRequest(r)
			if ok {
				timeUntilExpiry := time.Until(peer.ExpiresAt)
				if timeUntilExpiry < warningThreshold {
					log.Printf("WARNING: Client cert expires soon! ID=%s, expires in %s",
						peer.ID.String(), timeUntilExpiry)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Example HTTP handler that uses peer from context
func helloHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve peer from context (set by middleware)
	peer, ok := spiffehttp.PeerFromContext(r.Context())
	if !ok {
		// This should never happen if auth middleware ran.
		// Return 401 in case handler was mis-wired without middleware.
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Use peer for application logic
	fmt.Fprintf(w, "Hello, %s!\n", peer.ID.String())
	fmt.Fprintf(w, "Trust Domain: %s\n", peer.ID.TrustDomain().Name())
	fmt.Fprintf(w, "Path: %s\n", peer.ID.Path())
	fmt.Fprintf(w, "Certificate expires: %s\n", peer.ExpiresAt)
}

// Example showing how to use the middleware
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create SPIRE source
	source, err := spire.NewSource(ctx, spire.Config{})
	if err != nil {
		log.Fatalf("Failed to create SPIRE source: %v", err)
	}
	defer source.Close()

	// Get X509Source for TLS config
	x509Source := source.X509Source()

	// Create server TLS config
	tlsConfig, err := spiffehttp.NewServerTLSConfig(
		ctx,
		x509Source,
		x509Source,
		spiffehttp.ServerConfig{},
	)
	if err != nil {
		log.Fatalf("Failed to create TLS config: %v", err)
	}

	// Setup HTTP handlers with middleware
	mux := http.NewServeMux()

	// Public endpoint (no auth)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "OK")
	})

	// Protected endpoint with basic auth middleware
	protectedHandler := authMiddleware(http.HandlerFunc(helloHandler))
	mux.Handle("/api/hello", protectedHandler)

	// Protected endpoint with trust domain check
	allowedTD, err := spiffeid.TrustDomainFromString("example.org")
	if err != nil {
		log.Fatalf("Invalid trust domain: %v", err)
	}
	restrictedHandler := trustDomainMiddleware(allowedTD)(http.HandlerFunc(helloHandler))
	mux.Handle("/api/restricted", restrictedHandler)

	// Chain multiple middleware
	monitoredHandler := certExpiryWarningMiddleware(5 * time.Minute)(protectedHandler)
	mux.Handle("/api/monitored", monitoredHandler)

	// Start HTTPS server
	server := &http.Server{
		Addr:              ":8443",
		Handler:           mux,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
	}

	go func() {
		log.Println("Starting mTLS server on :8443")
		log.Println("Endpoints:")
		log.Println("  GET /health          - Public health check")
		log.Println("  GET /api/hello       - Basic auth (any peer)")
		log.Println("  GET /api/restricted  - Trust domain auth (example.org only)")
		log.Println("  GET /api/monitored   - With cert expiry warnings")

		if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Server stopped")
}
