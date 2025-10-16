package main

import (
	"context"
	"encoding/json"
	"errors"
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
	// Use signal.NotifyContext for cleaner cancellation wiring
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Load configuration from environment with SPIFFE-compatible naming
	cfg := loadConfig()

	// Create the mTLS server
	log.Println("Creating mTLS server with configuration:")
	log.Printf("  Socket: %s", cfg.WorkloadAPI.SocketPath)
	log.Printf("  Address: %s", cfg.HTTP.Address)
	log.Printf("  Allowed peer: %s", getAllowedPeer(cfg))

	server, err := identityserver.New(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer func() {
		log.Println("Closing server resources...")
		if err := server.Close(); err != nil {
			log.Printf("Error closing server: %v", err)
		}
	}()

	// Register handlers (fail fast on registration errors)
	registerHandlers(server)

	log.Println("✓ Server created and handlers registered successfully")
	log.Printf("Listening on %s with mTLS authentication", cfg.HTTP.Address)
	log.Println("Press Ctrl+C to stop")
	log.Println()

	// Start server in goroutine (blocks until shutdown)
	go func() {
		if err := server.Start(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	log.Println("Shutdown signal received, stopping server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}

	log.Println("Server stopped gracefully")
}

// loadConfig loads configuration from environment variables.
// Follows SPIFFE naming conventions (SPIFFE_ENDPOINT_SOCKET) with fallbacks.
func loadConfig() ports.MTLSConfig {
	var cfg ports.MTLSConfig

	// Socket path: prefer SPIFFE_ENDPOINT_SOCKET, fallback to SPIRE_AGENT_SOCKET
	cfg.WorkloadAPI.SocketPath = getFirstNonEmpty(
		os.Getenv("SPIFFE_ENDPOINT_SOCKET"),
		getEnv("SPIRE_AGENT_SOCKET", "unix:///tmp/spire-agent/public/api.sock"),
	)

	// SPIFFE authorization: support single ID or trust domain
	cfg.SPIFFE.AllowedPeerID = os.Getenv("ALLOWED_CLIENT_ID")
	cfg.SPIFFE.AllowedTrustDomain = os.Getenv("ALLOWED_TRUST_DOMAIN")

	// If neither set, default to allowing any client from example.org
	if cfg.SPIFFE.AllowedPeerID == "" && cfg.SPIFFE.AllowedTrustDomain == "" {
		cfg.SPIFFE.AllowedPeerID = "spiffe://example.org/client"
	}

	// HTTP server configuration
	cfg.HTTP.Address = getEnv("SERVER_ADDRESS", ":8443")
	cfg.HTTP.ReadHeaderTimeout = 10 * time.Second
	cfg.HTTP.ReadTimeout = 15 * time.Second
	cfg.HTTP.WriteTimeout = 30 * time.Second
	cfg.HTTP.IdleTimeout = 120 * time.Second

	return cfg
}

// registerHandlers registers all HTTP handlers with the server.
func registerHandlers(server ports.MTLSServer) {
	handlers := []struct {
		pattern string
		handler http.Handler
	}{
		{"/", http.HandlerFunc(handleRoot)},
		{"/api/hello", http.HandlerFunc(handleHello)},
		{"/api/identity", http.HandlerFunc(handleIdentity)},
		{"/health", http.HandlerFunc(handleHealth)},
	}

	for _, h := range handlers {
		if err := server.Handle(h.pattern, h.handler); err != nil {
			log.Fatalf("Failed to register handler for %s: %v", h.pattern, err)
		}
	}
}

// getAllowedPeer returns a display string for the configured peer authorization.
func getAllowedPeer(cfg ports.MTLSConfig) string {
	if cfg.SPIFFE.AllowedPeerID != "" {
		return cfg.SPIFFE.AllowedPeerID
	}
	if cfg.SPIFFE.AllowedTrustDomain != "" {
		return fmt.Sprintf("any from trust domain: %s", cfg.SPIFFE.AllowedTrustDomain)
	}
	return "any from trust domain"
}

// handleRoot is the root handler
func handleRoot(w http.ResponseWriter, r *http.Request) {
	// Use port-level identity accessor (adapter-agnostic)
	id, ok := ports.IdentityFrom(r.Context())
	if !ok {
		http.Error(w, "No identity", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Success! Authenticated as: %s\n", id.SPIFFEID)
	log.Printf("Root request from: %s", id.SPIFFEID)
}

// handleHello is a simple greeting handler with JSON response
func handleHello(w http.ResponseWriter, r *http.Request) {
	// Use port-level identity accessor (adapter-agnostic)
	id, ok := ports.IdentityFrom(r.Context())
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "No identity")
		return
	}

	response := map[string]string{
		"message":  "Hello from mTLS server!",
		"identity": id.SPIFFEID,
	}

	writeJSON(w, http.StatusOK, response)
	log.Printf("Hello request from: %s", id.SPIFFEID)
}

// handleIdentity returns detailed identity information in JSON format
func handleIdentity(w http.ResponseWriter, r *http.Request) {
	// Use port-level identity accessor (adapter-agnostic)
	id, ok := ports.IdentityFrom(r.Context())
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "No identity")
		return
	}

	response := map[string]interface{}{
		"identity": map[string]string{
			"spiffe_id":    id.SPIFFEID,
			"trust_domain": id.TrustDomain,
			"path":         id.Path,
		},
		"request": map[string]string{
			"method":      r.Method,
			"url":         r.URL.String(),
			"remote_addr": r.RemoteAddr,
		},
	}

	writeJSON(w, http.StatusOK, response)
	log.Printf("Identity request from: %s", id.SPIFFEID)
}

// handleHealth is a health check endpoint (no authentication required)
func handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "healthy",
	}
	writeJSON(w, http.StatusOK, response)
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getFirstNonEmpty returns the first non-empty string from the arguments
func getFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
