package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sufield/e5s"
)

// Version information (set via ldflags during build)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	// Default to example config for demonstration
	configPath := flag.String("config", "examples/highlevel/e5s.yaml", "Path to e5s config file")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("e5s-example-server %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
		os.Exit(0)
	}

	log.Printf("e5s mTLS server (version %s)", version)
	log.Printf("Using config: %s", *configPath)

	// Set config path for e5s.Serve to use
	// This allows the binary to own the default, not the library
	os.Setenv("E5S_CONFIG", *configPath)

	// Create HTTP router
	r := chi.NewRouter()

	// Health check endpoint
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			log.Printf("❌ write error: %v", err)
		}
	})

	// Authenticated endpoint that returns server time
	r.Get("/time", func(w http.ResponseWriter, req *http.Request) {
		// Extract peer identity from mTLS connection
		id, ok := e5s.PeerID(req)
		if !ok {
			log.Printf("❌ Unauthorized request from %s", req.RemoteAddr)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		log.Printf("✓ Authenticated request from: %s", id)

		// Get current server time
		serverTime := time.Now().Format(time.RFC3339)
		response := fmt.Sprintf("Server time: %s", serverTime)
		log.Printf("→ Sending response: %s", response)
		if _, err := fmt.Fprintf(w, "%s\n", response); err != nil {
			log.Printf("❌ write error: %v", err)
		}
	})

	// Serve handles startup, signal handling, and graceful shutdown
	if err := e5s.Serve(r); err != nil {
		log.Fatal(err)
	}
}
