package e5s_test

import (
	"fmt"
	"log"
	"net/http"

	"github.com/sufield/e5s"
)

// ExampleStart demonstrates starting an mTLS server with explicit
// configuration path and shutdown control.
func ExampleStart() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := e5s.PeerID(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, "Hello, %s!\n", id)
	})

	// Start server with explicit config path
	shutdown, err := e5s.Start("e5s.prod.yaml", handler)
	if err != nil {
		log.Fatal(err)
	}

	// Server is now running in background
	// Call shutdown() when you want to stop gracefully
	defer func() {
		if err := shutdown(); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
	}()

	// ... do other work ...
	// Server continues running until shutdown() is called
	log.Println("Server is running")
}

// ExamplePeerID demonstrates extracting the authenticated client's
// SPIFFE ID from an HTTP request.
func ExamplePeerID() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract peer's SPIFFE ID
		id, ok := e5s.PeerID(r)
		if !ok {
			// No valid mTLS identity - reject request
			http.Error(w, "Unauthorized: no valid SPIFFE identity", http.StatusUnauthorized)
			return
		}

		// Use the SPIFFE ID for authorization
		fmt.Fprintf(w, "Authenticated as: %s\n", id)

		// Check specific identity
		if id == "spiffe://example.org/admin" {
			fmt.Fprintf(w, "Admin access granted\n")
		}
	})

	_ = handler // Use in server
}

// ExamplePeerInfo demonstrates accessing full peer information
// including certificates.
func ExamplePeerInfo() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get complete peer information
		peer, ok := e5s.PeerInfo(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Access SPIFFE ID
		fmt.Fprintf(w, "Peer ID: %s\n", peer.ID)

		// Access trust domain
		trustDomain := peer.ID.TrustDomain().Name()
		fmt.Fprintf(w, "Trust domain: %s\n", trustDomain)

		// Access certificate expiration
		fmt.Fprintf(w, "Certificate expires: %s\n", peer.ExpiresAt)
	})

	_ = handler // Use in server
}

// ExampleClient demonstrates creating an HTTP client for making
// mTLS requests using configuration from a file.
func ExampleClient() {
	// Create mTLS-enabled HTTP client
	client, cleanup, err := e5s.Client("e5s.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Use the client for mTLS requests
	resp, err := client.Get("https://secure-service:8443/api")
	if err != nil {
		cleanup() // Clean up before exiting
		log.Fatal(err)
	}
	resp.Body.Close()
	cleanup()

	// Process response...
	fmt.Printf("Status: %d\n", resp.StatusCode)
}

// Example_authorizationByTrustDomain demonstrates authorizing requests
// based on trust domain.
func Example_authorizationByTrustDomain() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, ok := e5s.PeerInfo(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check trust domain
		trustDomain := peer.ID.TrustDomain().Name()
		if trustDomain != "example.org" {
			http.Error(w, "Forbidden: untrusted domain", http.StatusForbidden)
			return
		}

		fmt.Fprintf(w, "Access granted for %s\n", peer.ID)
	})

	_ = handler // Use in server
}

