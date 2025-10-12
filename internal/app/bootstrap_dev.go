//go:build dev

package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Bootstrap creates and wires all application components for development mode.
// This version supports in-memory implementations with registry seeding and attestor configuration.
//
// Development mode:
// - Uses AdapterFactory (full composite interface)
// - Creates in-memory registry and attestor
// - Seeds registry with identity mappers from configuration
// - Registers workload UIDs with attestor
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, factory ports.AdapterFactory) (*Application, error) {
	// Step 1: Load configuration (fixtures)
	config, err := configLoader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Step 2: Initialize development-specific components (registry + attestor)
	registry := factory.CreateRegistry()
	attestor := factory.CreateAttestor()

	// Step 3: Initialize trust domain parser (abstracts SDK trust domain validation)
	trustDomainParser := factory.CreateTrustDomainParser()

	// Step 4: Initialize identity credential parser (abstracts SDK parsing logic)
	parser := factory.CreateIdentityCredentialParser()

	// Step 5: Initialize identity document provider (abstracts SDK document creation/validation)
	docProvider := factory.CreateIdentityDocumentProvider()

	// Step 6: Initialize SPIRE server
	server, err := factory.CreateDevelopmentServer(ctx, ports.DevelopmentServerConfig{
		TrustDomain:       config.TrustDomain,
		TrustDomainParser: trustDomainParser,
		DocProvider:       docProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE server: %w", err)
	}

	// Step 7: SEED workload UID mappings (configuration, not runtime)
	for _, workload := range config.Workloads {
		factory.RegisterWorkloadUID(attestor, workload.UID, workload.Selector)
	}

	// Step 8: SEED registry with identity mappers (configuration, not runtime)
	for _, workload := range config.Workloads {
		// Parse identity credential from fixture
		identityCredential, err := parser.ParseFromString(ctx, workload.SpiffeID)
		if err != nil {
			return nil, fmt.Errorf("invalid identity credential %s: %w", workload.SpiffeID, err)
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
		mapper, err := domain.NewIdentityMapper(identityCredential, selectorSet)
		if err != nil {
			return nil, fmt.Errorf("failed to create identity mapper for %s: %w", workload.SpiffeID, err)
		}

		// SEED registry (internal method, not exposed via port)
		if err := factory.SeedRegistry(registry, ctx, mapper); err != nil {
			return nil, fmt.Errorf("failed to seed registry for %s: %w", workload.SpiffeID, err)
		}
	}

	// Step 9: SEAL registry (prevent further mutations after seeding)
	factory.SealRegistry(registry)

	// Step 10: Initialize SPIRE agent with registry
	// Development uses CreateDevelopmentAgent with config struct (all dependencies)
	agent, err := factory.CreateDevelopmentAgent(ctx, ports.DevelopmentAgentConfig{
		SPIFFEID:    config.AgentSpiffeID,
		Server:      server,
		Registry:    registry,
		Attestor:    attestor,
		Parser:      parser,
		DocProvider: docProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE agent: %w", err)
	}

	// Step 11: Initialize Identity Client service (server-side SVID issuance)
	identityClientService := NewIdentityClientService(agent)

	// Step 12: Initialize core service (demonstration use case)
	service := NewIdentityService(agent, registry)

	return &Application{
		Config:                config,
		Service:               service,
		IdentityClientService: identityClientService,
		Agent:                 agent,
		Registry:              registry,
	}, nil
}
