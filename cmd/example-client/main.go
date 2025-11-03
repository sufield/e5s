package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/sufield/e5s"
)

func main() {
	log.Println("Starting e5s mTLS client...")

	// Get server URL from environment variable, default to localhost
	// This allows the client to work both locally and in Kubernetes
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "https://localhost:8443/time"
	}

	log.Printf("→ Requesting server time from: %s", serverURL)
	log.Println("→ Initializing SPIRE client and fetching SPIFFE identity...")

	// Perform mTLS GET request (uses local e5s code)
	resp, err := e5s.Get(serverURL)
	if err != nil {
		log.Fatalf("❌ Request failed: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("✓ Received response: HTTP %d %s", resp.StatusCode, resp.Status)

	// Read and print response
	body, _ := io.ReadAll(resp.Body)
	log.Printf("← Server response: %s", string(body))
	fmt.Print(string(body))
}
