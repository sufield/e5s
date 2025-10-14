//go:build dev

package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Bootstrap creates and wires all application components for development mode.
//
// Development mode:
// - Uses concrete InMemoryAdapterFactory (no interfaces)
// - Components are pre-configured from config when created
// - Simple wiring of components
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, factory *compose.InMemoryAdapterFactory) (*Application, error) {
	// Step 1: Load configuration (fixtures)
	config, err := configLoader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Step 2: Initialize parsers and providers
	trustDomainParser := factory.CreateTrustDomainParser()
	parser := factory.CreateIdentityCredentialParser()
	docProvider := factory.CreateIdentityDocumentProvider()

	// Step 3: Initialize server
	server, err := factory.CreateServer(ctx, config.TrustDomain, trustDomainParser, docProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE server: %w", err)
	}

	// Step 4: Initialize registry (seeded with workload configurations)
	registry, err := factory.CreateRegistry(ctx, config.Workloads, parser)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry: %w", err)
	}

	// Step 5: Initialize attestor (configured with workload UIDs)
	attestor := factory.CreateAttestor(config.Workloads)

	// Step 6: Initialize agent
	agent, err := factory.CreateAgent(ctx, config.AgentSpiffeID, server, registry, attestor, parser, docProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE agent: %w", err)
	}

	// Step 7: Initialize services
	identityClientService := NewIdentityClientService(agent)
	service := NewIdentityService(agent, registry)

	return &Application{
		Config:                config,
		Service:               service,
		IdentityClientService: identityClientService,
		Agent:                 agent,
		Registry:              registry,
	}, nil
}
