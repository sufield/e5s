//go:build dev

package main

import (
	"context"
	"log"

	"github.com/pocket/hexagon/spire/internal/adapters/inbound/cli"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/app"
)

func main() {
	ctx := context.Background()

	// Step 1: Create configuration loader (outbound adapter)
	configLoader := inmemory.NewInMemoryConfig()

	// Step 2: Create adapter factory (infrastructure)
	factory := compose.NewInMemoryAdapterFactory()

	// Step 3: Bootstrap the application (composition root)
	application, err := app.Bootstrap(ctx, configLoader, factory)
	if err != nil {
		log.Fatalf("Failed to bootstrap application: %v", err)
	}

	// Step 4: Create CLI inbound adapter with bootstrapped application
	cliAdapter := cli.New(application)

	// Step 5: Run the application via CLI adapter
	if err := cliAdapter.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
