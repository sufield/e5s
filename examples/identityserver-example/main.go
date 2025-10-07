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
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Printf("\nReceived signal: %v\n", sig)
		fmt.Println("Initiating graceful shutdown...")
		cancel()
	}()

	// Load configuration from environment
	var cfg ports.MTLSConfig
	cfg.WorkloadAPI.SocketPath = getEnv("SPIRE_AGENT_SOCKET", "unix:///tmp/spire-agent/public/api.sock")
	cfg.SPIFFE.AllowedPeerID = getEnv("ALLOWED_CLIENT_ID", "spiffe://example.org/client")
	cfg.HTTP.Address = getEnv("SERVER_ADDRESS", ":8443")
	cfg.HTTP.ReadHeaderTimeout = 10 * time.Second
	cfg.HTTP.WriteTimeout = 30 * time.Second
	cfg.HTTP.IdleTimeout = 120 * time.Second

	// Create the mTLS server
	log.Println("Creating mTLS server with configuration:")
	log.Printf("  Socket: %s", cfg.WorkloadAPI.SocketPath)
	log.Printf("  Address: %s", cfg.HTTP.Address)
	log.Printf("  Allowed client: %s", cfg.SPIFFE.AllowedPeerID)

	server, err := identityserver.NewSPIFFEServer(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer func() {
		log.Println("Closing server resources...")
		if err := server.Close(); err != nil {
			log.Printf("Error closing server: %v", err)
		}
	}()

	// Register handlers
	server.Handle("/", http.HandlerFunc(handleRoot))
	server.Handle("/api/hello", http.HandlerFunc(handleHello))
	server.Handle("/api/identity", http.HandlerFunc(handleIdentity))
	server.Handle("/health", http.HandlerFunc(handleHealth))

	log.Println("✓ Server created successfully")
	log.Printf("Listening on %s with mTLS authentication", cfg.HTTP.Address)
	log.Println("Press Ctrl+C to stop")
	log.Println()

	// Start server (blocks until context is cancelled)
	if err := server.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped gracefully")
}

// handleRoot is the root handler
func handleRoot(w http.ResponseWriter, r *http.Request) {
	clientID, ok := identityserver.GetSPIFFEID(r)
	if !ok {
		http.Error(w, "No identity", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Success! Authenticated as: %s\n", clientID.String())
	log.Printf("Root request from: %s", clientID.String())
}

// handleHello is a simple greeting handler
func handleHello(w http.ResponseWriter, r *http.Request) {
	clientID, ok := identityserver.GetSPIFFEID(r)
	if !ok {
		http.Error(w, "No identity", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Hello from mTLS server!\n")
	fmt.Fprintf(w, "Authenticated client: %s\n", clientID.String())
	log.Printf("Hello request from: %s", clientID.String())
}

// handleIdentity returns detailed identity information
func handleIdentity(w http.ResponseWriter, r *http.Request) {
	clientID, ok := identityserver.GetSPIFFEID(r)
	if !ok {
		http.Error(w, "No identity", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "=== Client Identity Details ===\n")
	fmt.Fprintf(w, "SPIFFE ID: %s\n", clientID.String())
	fmt.Fprintf(w, "Trust Domain: %s\n", clientID.TrustDomain().String())
	fmt.Fprintf(w, "Path: %s\n", clientID.Path())
	fmt.Fprintf(w, "\n=== Request Details ===\n")
	fmt.Fprintf(w, "Method: %s\n", r.Method)
	fmt.Fprintf(w, "URL: %s\n", r.URL.String())
	fmt.Fprintf(w, "Remote Addr: %s\n", r.RemoteAddr)

	log.Printf("Identity request from: %s", clientID.String())
}

// handleHealth is a health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"healthy"}`)
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
