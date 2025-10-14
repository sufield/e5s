//go:build !dev

package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Bootstrap creates and wires all application components for production mode.
//
// Production mode:
// - Uses SPIREAdapterFactory to connect to external SPIRE infrastructure
// - Delegates identity operations to SPIRE Agent/Server via Workload API
// - No in-memory registry or attestor - relies on SPIRE registration entries
// - Simple wiring focused on production components only
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, factory *compose.SPIREAdapterFactory) (*Application, error) {
	// Step 1: Load configuration
	config, err := configLoader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Step 2: Initialize parsers
	parser := factory.CreateIdentityCredentialParser()

	// Step 3: Initialize agent (connects to SPIRE via Workload API)
	agent, err := factory.CreateAgent(ctx, config.AgentSpiffeID, parser)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE agent: %w", err)
	}

	// Step 4: Initialize services
	identityClientService := NewIdentityClientService(agent)

	return &Application{
		Config:                config,
		IdentityClientService: identityClientService,
		Agent:                 agent,
		// No Registry or demo Service in production
	}, nil
}
