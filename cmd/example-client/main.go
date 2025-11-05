package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/sufield/e5s"
	"github.com/sufield/e5s/internal/config"
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
		fmt.Printf("e5s-example-client %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
		os.Exit(0)
	}

	os.Exit(run(*configPath))
}

func run(configPath string) int {
	log.Printf("Starting e5s mTLS client (version %s)...", version)
	log.Printf("Using config: %s", configPath)

	// Load config to get server URL
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("❌ Failed to load config: %v", err)
		return 1
	}

	// Get server URL from environment variable, or fall back to config
	// This allows the client to work both locally and in Kubernetes
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = cfg.Client.ServerURL
	}
	if serverURL == "" {
		log.Printf("❌ server_url not set in config and SERVER_URL environment variable not set")
		return 1
	}

	log.Printf("→ Requesting server time from: %s", serverURL)
	log.Println("→ Initializing SPIRE client and fetching SPIFFE identity...")

	// Create mTLS client with explicit config path
	// This allows the binary to own the default, not the library
	client, shutdown, err := e5s.Client(configPath)
	if err != nil {
		log.Printf("❌ Failed to initialize client: %v", err)
		return 1
	}
	defer func() {
		if err := shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	// Perform mTLS GET request
	resp, err := client.Get(serverURL)
	if err != nil {
		log.Printf("❌ Request failed: %v", err)
		return 1
	}
	defer resp.Body.Close()

	log.Printf("✓ Received response: HTTP %d %s", resp.StatusCode, resp.Status)

	// Read and print response
	body, _ := io.ReadAll(resp.Body)
	log.Printf("← Server response: %s", string(body))
	fmt.Print(string(body))

	return 0
}
