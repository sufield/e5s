package spiffehttp_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/sufield/e5s/spiffehttp"
)

// ExampleNewServerTLSConfig demonstrates creating a server TLS configuration
// that accepts mTLS connections from any client in the same trust domain.
//
// This example requires a running SPIRE agent.
func ExampleNewServerTLSConfig() {
	ctx := context.Background()

	// Connect to SPIRE Workload API
	source, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		log.Fatalf("Unable to create X509Source: %v", err)
	}
	defer source.Close()

	// Create server TLS config that accepts any client in same trust domain
	tlsConfig, err := spiffehttp.NewServerTLSConfig(
		ctx,
		source,
		source,
		spiffehttp.ServerConfig{},
	)
	if err != nil {
		source.Close() // Clean up before exiting
		log.Fatalf("Unable to create TLS config: %v", err)
	}

	// Use the TLS config in an HTTP server
	server := &http.Server{
		Addr:      ":8443",
		TLSConfig: tlsConfig,
	}

	log.Printf("Server listening on %s", server.Addr)
	_ = server // Use in production: server.ListenAndServeTLS("", "")
}

// ExampleNewServerTLSConfig_specificClient demonstrates restricting server
// to accept connections from only a specific SPIFFE ID.
func ExampleNewServerTLSConfig_specificClient() {
	ctx := context.Background()

	source, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		log.Fatalf("Unable to create X509Source: %v", err)
	}
	defer source.Close()

	// Only accept connections from specific client SPIFFE ID
	tlsConfig, err := spiffehttp.NewServerTLSConfig(
		ctx,
		source,
		source,
		spiffehttp.ServerConfig{
			AllowedClientID: "spiffe://example.org/api-client",
		},
	)
	if err != nil {
		source.Close() // Clean up before exiting
		log.Fatalf("Unable to create TLS config: %v", err)
	}

	server := &http.Server{
		Addr:      ":8443",
		TLSConfig: tlsConfig,
	}
	_ = server // Use server
}

// ExampleNewServerTLSConfig_trustDomain demonstrates accepting clients
// from a specific trust domain (useful for federation scenarios).
func ExampleNewServerTLSConfig_trustDomain() {
	ctx := context.Background()

	source, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		log.Fatalf("Unable to create X509Source: %v", err)
	}
	defer source.Close()

	// Accept any client from partner trust domain
	tlsConfig, err := spiffehttp.NewServerTLSConfig(
		ctx,
		source,
		source,
		spiffehttp.ServerConfig{
			AllowedClientTrustDomain: "partner.example.org",
		},
	)
	if err != nil {
		source.Close() // Clean up before exiting
		log.Fatalf("Unable to create TLS config: %v", err)
	}

	server := &http.Server{
		Addr:      ":8443",
		TLSConfig: tlsConfig,
	}
	_ = server // Use server
}

// ExampleNewClientTLSConfig demonstrates creating a client TLS configuration
// that verifies the server's SPIFFE identity.
//
// This example requires a running SPIRE agent.
func ExampleNewClientTLSConfig() {
	ctx := context.Background()

	// Connect to SPIRE Workload API
	source, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		log.Fatalf("Unable to create X509Source: %v", err)
	}
	defer source.Close()

	// Create client TLS config that verifies any server in trust domain
	tlsConfig, err := spiffehttp.NewClientTLSConfig(
		ctx,
		source,
		source,
		spiffehttp.ClientConfig{
			ExpectedServerTrustDomain: "example.org",
		},
	)
	if err != nil {
		log.Fatalf("Unable to create TLS config: %v", err)
	}

	// Use the TLS config in an HTTP client
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	resp, err := client.Get("https://server.example.org:8443")
	if err != nil {
		source.Close() // Clean up before exiting
		log.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()
}

// ExampleNewClientTLSConfig_specificServer demonstrates verifying
// a specific server SPIFFE ID.
func ExampleNewClientTLSConfig_specificServer() {
	ctx := context.Background()

	source, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		log.Fatalf("Unable to create X509Source: %v", err)
	}
	defer source.Close()

	// Only connect to servers with specific SPIFFE ID
	tlsConfig, err := spiffehttp.NewClientTLSConfig(
		ctx,
		source,
		source,
		spiffehttp.ClientConfig{
			ExpectedServerID: "spiffe://example.org/api-server",
		},
	)
	if err != nil {
		source.Close() // Clean up before exiting
		log.Fatalf("Unable to create TLS config: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	_ = client // Use client
}

// ExamplePeerFromRequest demonstrates extracting the authenticated peer's
// SPIFFE identity from an HTTP request.
func ExamplePeerFromRequest() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract peer identity from mTLS connection
		peer, ok := spiffehttp.PeerFromRequest(r)
		if !ok {
			http.Error(w, "Unauthorized: no valid SPIFFE identity", http.StatusUnauthorized)
			return
		}

		// Use the peer's SPIFFE ID for authorization
		fmt.Fprintf(w, "Hello, %s!\n", peer.ID)

		// Access trust domain
		trustDomain := peer.ID.TrustDomain().Name()
		fmt.Fprintf(w, "Trust domain: %s\n", trustDomain)

		// Check if peer matches expected ID
		expectedID, _ := spiffeid.FromString("spiffe://example.org/api-client")
		if peer.ID == expectedID {
			fmt.Fprintf(w, "Authorized client\n")
		}
	})

	_ = handler // Use handler in server
}

// ExampleWithPeer demonstrates attaching peer information to a request context
// for use in middleware chains.
func ExampleWithPeer() {
	// Middleware that extracts peer and attaches to context
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer, ok := spiffehttp.PeerFromRequest(r)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Attach peer to context for downstream handlers
			ctx := spiffehttp.WithPeer(r.Context(), peer)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Handler that retrieves peer from context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, ok := spiffehttp.PeerFromContext(r.Context())
		if !ok {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Peer: %s\n", peer.ID)
	})

	// Chain middleware
	_ = authMiddleware(handler)
}

// ExamplePeerFromContext demonstrates retrieving peer information from
// a request context (typically set by middleware).
func ExamplePeerFromContext() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve peer from context (set by middleware)
		peer, ok := spiffehttp.PeerFromContext(r.Context())
		if !ok {
			// Peer not in context - middleware didn't run or auth failed
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Use peer for authorization decisions
		if peer.ID.TrustDomain().Name() != "example.org" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		fmt.Fprintf(w, "Authorized: %s\n", peer.ID)
	})

	_ = handler // Use handler
}

// Example_tlsVersionCheck demonstrates verifying the TLS version
// enforced by the library.
func Example_tlsVersionCheck() {
	ctx := context.Background()
	source, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	tlsConfig, err := spiffehttp.NewServerTLSConfig(
		ctx, source, source,
		spiffehttp.ServerConfig{},
	)
	if err != nil {
		source.Close() // Clean up before exiting
		log.Fatal(err)
	}

	// Verify TLS 1.3 is enforced
	if tlsConfig.MinVersion == tls.VersionTLS13 {
		fmt.Println("TLS 1.3 enforced")
	}
}
