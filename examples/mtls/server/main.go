package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/spiffetls"
)

func main() {
	ctx := context.Background()

	// Create identity server configuration
	// Using defaults: socket path, address, timeouts
	cfg := ports.DefaultMTLSConfig()

	// Optionally override from environment
	if socketPath := os.Getenv("SPIRE_AGENT_SOCKET"); socketPath != "" {
		cfg.WorkloadAPI.SocketPath = socketPath
	}
	if address := os.Getenv("SERVER_ADDRESS"); address != "" {
		cfg.HTTP.Address = address
	}

	// Optionally restrict to specific client SPIFFE ID
	// If not set, any client from the trust domain is allowed
	if allowedClientID := os.Getenv("ALLOWED_CLIENT_ID"); allowedClientID != "" {
		cfg.SPIFFE.AllowedPeerID = allowedClientID
	}

	log.Printf("Starting mTLS server with configuration:")
	log.Printf("  Socket: %s", cfg.WorkloadAPI.SocketPath)
	log.Printf("  Address: %s", cfg.HTTP.Address)
	if cfg.SPIFFE.AllowedPeerID != "" {
		log.Printf("  Allowed client: %s", cfg.SPIFFE.AllowedPeerID)
	} else {
		log.Printf("  Allowed client: any from trust domain")
	}

	// Create identity server
	// This will:
	// - Connect to SPIRE agent
	// - Fetch server's X.509 SVID
	// - Configure mTLS with client authentication
	server, err := identityserver.NewSPIFFEServer(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create identity server: %v", err)
	}
	defer server.Close()

	// Register handlers
	// The server performs authentication - handlers receive authenticated requests
	server.Handle("/api/hello", http.HandlerFunc(helloHandler))
	server.Handle("/api/echo", http.HandlerFunc(echoHandler))
	server.Handle("/health", http.HandlerFunc(healthHandler))

	// Start server
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("✓ Server started successfully on %s", cfg.HTTP.Address)
	log.Printf("Waiting for requests (Ctrl+C to stop)...")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	} else {
		log.Println("✓ Server shutdown complete")
	}
}

// helloHandler demonstrates accessing the authenticated client's identity.
func helloHandler(w http.ResponseWriter, r *http.Request) {
	// Extract client's SPIFFE ID from mTLS connection
	// This is available because the server performed mTLS authentication
	clientID, err := spiffetls.PeerIDFromConnectionState(*r.TLS)
	if err != nil {
		// Should not happen - server already authenticated the client
		http.Error(w, "Failed to get peer identity", http.StatusInternalServerError)
		log.Printf("Error getting peer ID: %v", err)
		return
	}

	// Application can now use the authenticated identity
	// This is authentication only - authorization is application's responsibility
	log.Printf("Request from authenticated client: %s", clientID)

	// Example: Application-level authorization (out of scope for this library)
	// if !myAuthzService.IsAllowed(clientID, "read", "hello") {
	//     http.Error(w, "Forbidden", http.StatusForbidden)
	//     return
	// }

	// Respond with client identity
	response := fmt.Sprintf("Hello from mTLS server!\nAuthenticated client: %s\n", clientID)
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

// echoHandler echoes back the request body and client identity.
func echoHandler(w http.ResponseWriter, r *http.Request) {
	clientID, err := spiffetls.PeerIDFromConnectionState(*r.TLS)
	if err != nil {
		http.Error(w, "Failed to get peer identity", http.StatusInternalServerError)
		log.Printf("Error getting peer ID: %v", err)
		return
	}

	log.Printf("Echo request from: %s", clientID)

	// Echo back client identity
	response := fmt.Sprintf("Echo from server\nClient: %s\nMethod: %s\nPath: %s\n",
		clientID, r.Method, r.URL.Path)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response))
}

// healthHandler provides a health check endpoint.
// Note: This still requires mTLS authentication by the server.
// For monitoring that bypasses mTLS, run a separate non-mTLS server.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}
