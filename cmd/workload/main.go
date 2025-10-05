package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/workloadapi"
)

const defaultSocketPath = "/tmp/spire-agent/public/api.sock"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get socket path from environment or use default
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = defaultSocketPath
	}

	fmt.Println("=== Workload Fetching SVID ===")
	fmt.Printf("Process UID: %d\n", os.Getuid())
	fmt.Printf("Process PID: %d\n", os.Getpid())
	fmt.Printf("Connecting to: %s\n\n", socketPath)

	// Create Workload API client
	client := workloadapi.NewClient(socketPath)

	// Fetch X.509 SVID
	fmt.Println("Fetching X.509 SVID from agent...")
	svid, err := client.FetchX509SVID(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch SVID: %v", err)
	}

	// Display SVID details
	fmt.Println("✓ SVID fetched successfully!")
	fmt.Println()
	fmt.Printf("SPIFFE ID: %s\n", svid.GetSPIFFEID())
	fmt.Printf("Certificate: %s\n", svid.GetX509SVID())
	fmt.Printf("Expires At: %s\n", time.Unix(svid.GetExpiresAt(), 0).Format("2006-01-02 15:04:05"))
	fmt.Println()
	fmt.Println("✓ Workload successfully authenticated!")
}
