package main

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/pocket/hexagon/spire/pkg/zerotrustclient"
)

func main() {
	ctx := context.Background()

	// Create a zero-config mTLS client
	// Only specify the server's SPIFFE ID - everything else is auto-detected
	client, err := zerotrustclient.New(ctx, &zerotrustclient.Config{
		ServerID: "spiffe://example.org/server",
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Make a GET request
	// Note: The hostname doesn't matter for identity verification
	// Only the server's SPIFFE ID is verified
	resp, err := client.Get(ctx, "https://localhost:8443/api/hello")
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	fmt.Printf("Response (%d): %s\n", resp.StatusCode, body)
}
