package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pocket/hexagon/spire/pkg/zerotrustserver"
)

// rootHandler returns "Success!" only if the request context carries an identity.
func rootHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := zerotrustserver.PeerIdentity(r.Context())
	if !ok {
		log.Printf("⚠️  Unauthorized request from %s", r.RemoteAddr)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	log.Printf("✓ Authenticated request from %s (SPIFFE ID: %s)", r.RemoteAddr, id.SPIFFEID)
	fmt.Fprintf(w, "Success! Authenticated as: %s\n", id.SPIFFEID)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("========================================")
	log.Println("Zero Trust mTLS Server - Starting")
	log.Println("========================================")

	// Log configuration from environment
	socketPath := os.Getenv("SPIFFE_ENDPOINT_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///spire-socket/api.sock (auto-detected)"
	}
	serverAddr := os.Getenv("SERVER_ADDRESS")
	if serverAddr == "" {
		serverAddr = ":8443 (default)"
	}

	log.Printf("Configuration:")
	log.Printf("  SPIRE Socket: %s", socketPath)
	log.Printf("  Listen Address: %s", serverAddr)
	log.Println("========================================")

	// Cancel on SIGINT/SIGTERM for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	routes := map[string]http.Handler{
		"/": http.HandlerFunc(rootHandler),
	}

	log.Println("Connecting to SPIRE agent...")
	log.Println("Starting mTLS server...")

	if err := zerotrustserver.Serve(ctx, routes); err != nil {
		stop() // Ensure cleanup before exit
		//nolint:gocritic // exitAfterDefer: stop() called explicitly before Fatal
		log.Fatalf("❌ Server error: %v", err)
	}

	log.Println("========================================")
	log.Println("Server shutdown complete")
	log.Println("========================================")
}
