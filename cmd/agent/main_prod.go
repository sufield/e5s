//go:build !dev

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/inbound/workloadapi"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
	"github.com/pocket/hexagon/spire/internal/app"
)

const (
	defaultSocketPath  = "unix:///tmp/spire-agent/public/api.sock"
	defaultTrustDomain = "example.org"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("=== SPIRE Agent (Production Mode) ===")

	// Step 1: Get configuration from environment
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = defaultSocketPath
	}

	trustDomain := os.Getenv("SPIRE_TRUST_DOMAIN")
	if trustDomain == "" {
		trustDomain = defaultTrustDomain
	}

	fmt.Printf("Connecting to SPIRE Agent: %s\n", socketPath)
	fmt.Printf("Trust Domain: %s\n", trustDomain)

	// Step 2: Create SPIRE adapter factory (production mode)
	spireConfig := &spire.Config{
		SocketPath:  socketPath,
		TrustDomain: trustDomain,
		Timeout:     30 * time.Second,
	}

	factory, err := compose.NewSPIREAdapterFactory(ctx, spireConfig)
	if err != nil {
		log.Fatalf("Failed to create SPIRE adapter factory: %v", err)
	}
	defer factory.Close()

	// Step 3: Create configuration loader
	// NOTE: Using in-memory config for now - loads workload registrations from inmemory/config.go
	// In production, replace with real ConfigLoader that loads from:
	//   - File (JSON/YAML)
	//   - Environment variables
	//   - Consul/etcd
	//   - SPIRE Server API (for dynamic registration)
	configLoader := inmemory.NewInMemoryConfig()

	// Step 4: Bootstrap the application (composition root)
	application, err := app.Bootstrap(ctx, configLoader, factory)
	if err != nil {
		log.Fatalf("Failed to bootstrap application: %v", err)
	}

	// Step 5: Get Workload API socket path from environment
	workloadAPISocket := os.Getenv("WORKLOAD_API_SOCKET")
	if workloadAPISocket == "" {
		workloadAPISocket = "/tmp/spire-agent/public/api.sock" // Unix socket without unix:// prefix
	}

	// Step 6: Create and start Workload API server (inbound adapter)
	workloadAPIServer := workloadapi.NewServer(application.IdentityClientService, workloadAPISocket)
	if err := workloadAPIServer.Start(ctx); err != nil {
		log.Fatalf("Failed to start Workload API server: %v", err)
	}
	defer workloadAPIServer.Stop(ctx)

	fmt.Printf("Agent Identity: %s\n", application.Config.AgentSpiffeID)
	fmt.Printf("Workload API socket: %s\n", workloadAPISocket)
	fmt.Printf("Registered workloads: %d\n", len(application.Config.Workloads))
	for _, w := range application.Config.Workloads {
		fmt.Printf("  - %s (UID: %d)\n", w.SpiffeID, w.UID)
	}
	fmt.Println()
	fmt.Println("Agent is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-sigCh
	fmt.Println("\nShutting down agent...")
}
