package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pocket/hexagon/spire/internal/adapters/inbound/workloadapi"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/app"
)

const defaultSocketPath = "/tmp/spire-agent/public/api.sock"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Step 1: Create configuration loader (outbound adapter)
	configLoader := inmemory.NewInMemoryConfig()

	// Step 2: Create adapter factory (infrastructure)
	factory := compose.NewInMemoryAdapterFactory()

	// Step 3: Bootstrap the application (composition root)
	application, err := app.Bootstrap(ctx, configLoader, factory)
	if err != nil {
		log.Fatalf("Failed to bootstrap application: %v", err)
	}

	// Step 4: Get socket path from environment or use default
	socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
	if socketPath == "" {
		socketPath = defaultSocketPath
	}

	// Step 5: Create and start Workload API server (inbound adapter)
	workloadAPIServer := workloadapi.NewServer(application.IdentityClientService, socketPath)
	if err := workloadAPIServer.Start(ctx); err != nil {
		log.Fatalf("Failed to start Workload API server: %v", err)
	}
	defer workloadAPIServer.Stop(ctx)

	fmt.Println("=== SPIRE Agent with Workload API ===")
	fmt.Printf("Trust Domain: %s\n", application.Config.TrustDomain)
	fmt.Printf("Agent Identity: %s\n", application.Config.AgentSpiffeID)
	fmt.Printf("Workload API socket: %s\n", socketPath)
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
