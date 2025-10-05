package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Application is the composition root that wires all dependencies
// This is NOT an adapter - it's infrastructure/bootstrap logic
type Application struct {
	Config                *ports.Config
	Service               ports.Service
	IdentityClientService *IdentityClientService
	Agent                 ports.Agent
	Registry              ports.IdentityMapperRegistry
}

// Bootstrap creates and wires all application components
// This is where dependency injection and SEEDING happens
// Seeding is configuration, not runtime behavior - happens once at startup
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, factory ports.AdapterFactory) (*Application, error) {
	// Step 1: Load configuration (fixtures)
	config, err := configLoader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Step 2: Initialize registry (read-only at runtime, seeded at startup)
	registry := factory.CreateRegistry()

	// Step 3: Initialize trust domain parser (abstracts SDK trust domain validation)
	trustDomainParser := factory.CreateTrustDomainParser()

	// Step 4: Initialize identity namespace parser (abstracts SDK parsing logic)
	parser := factory.CreateIdentityNamespaceParser()

	// Step 5: Initialize identity document provider (abstracts SDK document creation/validation)
	docProvider := factory.CreateIdentityDocumentProvider()

	// Step 6: Initialize SPIRE server
	server, err := factory.CreateServer(ctx, config.TrustDomain, trustDomainParser, docProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE server: %w", err)
	}

	// Step 7: Initialize workload attestor
	attestor := factory.CreateAttestor()

	// Step 8: SEED workload UID mappings (configuration, not runtime)
	for _, workload := range config.Workloads {
		factory.RegisterWorkloadUID(attestor, workload.UID, workload.Selector)
	}

	// Step 9: SEED registry with identity mappers (configuration, not runtime)
	for _, workload := range config.Workloads {
		// Parse identity namespace from fixture
		identityNamespace, err := parser.ParseFromString(ctx, workload.SpiffeID)
		if err != nil {
			return nil, fmt.Errorf("invalid identity namespace %s: %w", workload.SpiffeID, err)
		}

		// Parse selectors from fixture
		selector, err := domain.ParseSelectorFromString(workload.Selector)
		if err != nil {
			return nil, fmt.Errorf("invalid selector %s: %w", workload.Selector, err)
		}

		// Create selector set for mapper
		selectorSet := domain.NewSelectorSet()
		selectorSet.Add(selector)

		// Create identity mapper (domain entity)
		mapper, err := domain.NewIdentityMapper(identityNamespace, selectorSet)
		if err != nil {
			return nil, fmt.Errorf("failed to create identity mapper for %s: %w", workload.SpiffeID, err)
		}

		// SEED registry (internal method, not exposed via port)
		if err := factory.SeedRegistry(registry, ctx, mapper); err != nil {
			return nil, fmt.Errorf("failed to seed registry for %s: %w", workload.SpiffeID, err)
		}
	}

	// Step 10: SEAL registry (prevent further mutations after seeding)
	factory.SealRegistry(registry)

	// Step 11: Initialize SPIRE agent with registry
	agent, err := factory.CreateAgent(ctx, config.AgentSpiffeID, server, registry, attestor, parser, docProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE agent: %w", err)
	}

	// Step 12: Initialize Identity Client service (server-side SVID issuance)
	identityClientService := NewIdentityClientService(agent)

	// Step 13: Initialize core service (demonstration use case)
	service := NewIdentityService(agent, registry)

	return &Application{
		Config:                config,
		Service:               service,
		IdentityClientService: identityClientService,
		Agent:                 agent,
		Registry:              registry,
	}, nil
}

