package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/app/ports"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// Application is the composition root that wires all dependencies
// This is NOT an adapter - it's infrastructure/bootstrap logic
type Application struct {
	Config   *ports.Config
	Service  ports.Service
	Agent    ports.Agent
	Registry ports.IdentityMapperRegistry
}

// Bootstrap creates and wires all application components
// This is where dependency injection and SEEDING happens
// Seeding is configuration, not runtime behavior - happens once at startup
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, deps ApplicationDeps) (*Application, error) {
	// Step 1: Load configuration (fixtures)
	config, err := configLoader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Step 2: Initialize registry (read-only at runtime, seeded at startup)
	registry := deps.CreateRegistry()

	// Step 3: Initialize trust domain parser (abstracts SDK trust domain validation)
	trustDomainParser := deps.CreateTrustDomainParser()

	// Step 4: Initialize identity namespace parser (abstracts SDK parsing logic)
	parser := deps.CreateIdentityNamespaceParser()

	// Step 5: Initialize identity document provider (abstracts SDK document creation/validation)
	docProvider := deps.CreateIdentityDocumentProvider()

	// Step 6: Initialize SPIRE server
	server, err := deps.CreateServer(ctx, config.TrustDomain, trustDomainParser, docProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE server: %w", err)
	}

	// Step 7: Initialize workload attestor
	attestor := deps.CreateAttestor()

	// Step 8: SEED workload UID mappings (configuration, not runtime)
	for _, workload := range config.Workloads {
		deps.RegisterWorkloadUID(attestor, workload.UID, workload.Selector)
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
		if err := deps.SeedRegistry(registry, ctx, mapper); err != nil {
			return nil, fmt.Errorf("failed to seed registry for %s: %w", workload.SpiffeID, err)
		}
	}

	// Step 10: SEAL registry (prevent further mutations after seeding)
	deps.SealRegistry(registry)

	// Step 11: Initialize SPIRE agent with registry
	agent, err := deps.CreateAgent(ctx, config.AgentSpiffeID, server, registry, attestor, parser, docProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE agent: %w", err)
	}

	// Step 12: Initialize core service
	service := NewIdentityService(agent, registry)

	return &Application{
		Config:   config,
		Service:  service,
		Agent:    agent,
		Registry: registry,
	}, nil
}

// ApplicationDeps abstracts the creation of concrete adapters
// This allows different implementations (in-memory, real SPIRE, etc.)
// Includes seeding methods for registry (configuration, not runtime behavior)
type ApplicationDeps interface {
	CreateRegistry() ports.IdentityMapperRegistry
	CreateTrustDomainParser() ports.TrustDomainParser
	CreateIdentityNamespaceParser() ports.IdentityNamespaceParser
	CreateIdentityDocumentProvider() ports.IdentityDocumentProvider
	CreateServer(ctx context.Context, trustDomain string, trustDomainParser ports.TrustDomainParser, docProvider ports.IdentityDocumentProvider) (ports.Server, error)
	CreateAttestor() ports.WorkloadAttestor
	RegisterWorkloadUID(attestor ports.WorkloadAttestor, uid int, selector string)
	CreateAgent(ctx context.Context, spiffeID string, server ports.Server, registry ports.IdentityMapperRegistry, attestor ports.WorkloadAttestor, parser ports.IdentityNamespaceParser, docProvider ports.IdentityDocumentProvider) (ports.Agent, error)

	// Seeding operations (configuration, not runtime - called only during bootstrap)
	SeedRegistry(registry ports.IdentityMapperRegistry, ctx context.Context, mapper *domain.IdentityMapper) error
	SealRegistry(registry ports.IdentityMapperRegistry)
}
