package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/pocket/hexagon/spire/examples/httpclient"
)

func main() {
	ctx := context.Background()

	// Create HTTP client configuration
	cfg := httpclient.DefaultConfig()

	// Optionally override from environment
	if socketPath := os.Getenv("SPIRE_AGENT_SOCKET"); socketPath != "" {
		cfg.WorkloadAPI.SocketPath = socketPath
	}

	// Optionally specify expected server SPIFFE ID
	// If not set, any server from the trust domain is accepted
	if expectedServerID := os.Getenv("EXPECTED_SERVER_ID"); expectedServerID != "" {
		cfg.SPIFFE.ExpectedServerID = expectedServerID
	}

	// Server URL from environment or default
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "https://localhost:8443"
	}

	log.Printf("Creating mTLS client with configuration:")
	log.Printf("  Socket: %s", cfg.WorkloadAPI.SocketPath)
	log.Printf("  Server URL: %s", serverURL)
	if cfg.SPIFFE.ExpectedServerID != "" {
		log.Printf("  Expected server: %s", cfg.SPIFFE.ExpectedServerID)
	} else {
		log.Printf("  Expected server: any from trust domain")
	}

	// Create HTTP client
	// This will:
	// - Connect to SPIRE agent
	// - Fetch client's X.509 SVID
	// - Configure mTLS with server authentication
	client, err := httpclient.New(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create HTTP client: %v", err)
	}
	defer client.Close()

	log.Printf("✓ Client created successfully")

	// Make GET request to /api/hello
	log.Printf("\n=== Making GET request to /api/hello ===")
	if err := makeGetRequest(ctx, client, serverURL+"/api/hello"); err != nil {
		log.Printf("❌ GET request failed: %v", err)
	} else {
		log.Printf("✓ GET request succeeded")
	}

	// Wait a bit
	time.Sleep(1 * time.Second)

	// Make GET request to /api/echo
	log.Printf("\n=== Making GET request to /api/echo ===")
	if err := makeGetRequest(ctx, client, serverURL+"/api/echo"); err != nil {
		log.Printf("❌ GET request failed: %v", err)
	} else {
		log.Printf("✓ GET request succeeded")
	}

	// Wait a bit
	time.Sleep(1 * time.Second)

	// Make GET request to /health
	log.Printf("\n=== Making GET request to /health ===")
	if err := makeGetRequest(ctx, client, serverURL+"/health"); err != nil {
		log.Printf("❌ Health check failed: %v", err)
	} else {
		log.Printf("✓ Health check succeeded")
	}

	log.Printf("\n✓ All requests completed successfully")
}

// makeGetRequest performs a GET request and prints the response.
func makeGetRequest(ctx context.Context, client httpclient.Client, url string) error {
	log.Printf("GET %s", url)

	resp, err := client.Get(ctx, url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("Status: %s", resp.Status)

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("Response:\n%s", string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
