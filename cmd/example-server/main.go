package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sufield/e5s"
)

func main() {
	log.Println("Starting e5s mTLS server...")

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

	log.Println("Server configured, initializing mTLS with SPIRE...")
	// Run mTLS server (uses local e5s code)
	e5s.Run(r)
}
