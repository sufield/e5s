//go:build dev

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/pocket/hexagon/spire/internal/adapters/inbound/cli"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/app"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== In-Memory SPIRE System Demo ===")
	fmt.Println()
	fmt.Println("This demonstrates hexagonal architecture with in-memory adapters.")
	fmt.Println("No infrastructure required - all components run in-process.")
	fmt.Println()

	// Step 1: Create configuration loader (outbound adapter)
	configLoader := inmemory.NewInMemoryConfig()

	// Step 2: Create adapter factory (infrastructure)
	factory := compose.NewInMemoryAdapterFactory()

	// Step 3: Bootstrap the application (composition root)
	// This wires all dependencies using hexagonal architecture:
	// - Domain entities (pure business logic)
	// - Ports (interfaces/contracts)
	// - Adapters (in-memory implementations)
	application, err := app.Bootstrap(ctx, configLoader, factory)
	if err != nil {
		log.Fatalf("Failed to bootstrap application: %v", err)
	}

	defer application.Close()

	fmt.Printf("✓ Application bootstrapped successfully\n")
	fmt.Printf("  Trust Domain: %s\n", application.Config().TrustDomain)
	fmt.Printf("  Agent Identity: %s\n", application.Config().AgentSpiffeID)
	fmt.Printf("  Registered Workloads: %d\n", len(application.Config().Workloads))
	for _, w := range application.Config().Workloads {
		fmt.Printf("    - %s (UID: %d)\n", w.SpiffeID, w.UID)
	}
	fmt.Println()

	// Step 4: Create CLI inbound adapter with bootstrapped application
	cliAdapter := cli.New(application)

	// Step 5: Run the application via CLI adapter
	if err := cliAdapter.Run(ctx); err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Println("=== Demo Complete ===")
	fmt.Println("Key Architecture Points:")
	fmt.Println("  - No external SPIRE infrastructure needed")
	fmt.Println("  - In-memory adapters implement same ports as production")
	fmt.Println("  - Domain logic is identical in both modes")
	fmt.Println("  - Swappable implementations via dependency injection")
}
