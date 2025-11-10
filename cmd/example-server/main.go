package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	configPath := flag.String("config", "examples/highlevel/e5s-server.yaml", "Path to e5s server config file")
	debug := flag.Bool("debug", false, "Enable debug mode (foreground, verbose logging)")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("e5s-example-server %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
		os.Exit(0)
	}

	// Enable debug logging if requested
	if *debug {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
		os.Setenv("E5S_DEBUG", "1")
		log.Println("⚠️  DEBUG MODE: verbose logging enabled")
	}

	log.Printf("e5s mTLS server (version %s)", version)
	log.Printf("Using config: %s", *configPath)

	// Check for debug mode (support both new -debug flag and legacy env var)
	debugMode := *debug || os.Getenv("E5S_DEBUG_SINGLE_THREAD") != ""
	if debugMode {
		log.Println("⚠️  DEBUG MODE: Running in single-threaded mode")
		log.Println("   This eliminates e5s's HTTP server goroutine for debugging")
	}

	// example-start:server-setup
	// Create HTTP router
	r := chi.NewRouter()
	// example-end:server-setup

	// Health check endpoint
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			log.Printf("❌ write error: %v", err)
		}
	})

	// example-start:authenticated-endpoint
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
	// example-end:authenticated-endpoint

	// example-start:server-start
	if debugMode {
		// Debug mode: use StartSingleThread (blocks, no goroutines from e5s)
		log.Println("Starting e5s mTLS server (single-threaded)...")
		if err := e5s.StartSingleThread(*configPath, r); err != nil {
			log.Fatal(err)
		}
	} else {
		// Normal mode: use Start (spawns goroutine, returns immediately)
		log.Println("Starting e5s mTLS server...")
		shutdown, err := e5s.Start(*configPath, r)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Server running - press Ctrl+C to stop")

		// Wait for interrupt signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down gracefully...")
		if err := shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
			os.Exit(1)
		}
	}
	// example-end:server-start
}
