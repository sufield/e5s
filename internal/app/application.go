package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Application is the composition root that wires all dependencies
// This is an infrastructure/bootstrap logic
type Application struct {
	Config                *ports.Config
	Service               ports.Service
	IdentityClientService *IdentityClientService
	Agent                 ports.Agent
	Registry              ports.IdentityMapperRegistry
}

// Bootstrap creates and wires all application components
// This is where dependency injection and Seeding happens
// Seeding is configuration, not runtime behavior - happens once at startup
//
// Supports both development and production modes:
// - Development: Uses full AdapterFactory with registry seeding
// - Production: Uses CoreAdapterFactory only (SPIRE handles registration)
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, factory ports.CoreAdapterFactory) (*Application, error) {
	// Step 1: Load configuration (fixtures)
	config, err := configLoader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Step 2: Check if factory provides development capabilities (registry + attestor)
	// In production (SPIREAdapterFactory), these will be nil
	// In development (InMemoryAdapterFactory), these will be created
	var registry ports.IdentityMapperRegistry
	var attestor ports.WorkloadAttestor

	if devFactory, ok := factory.(ports.DevelopmentAdapterFactory); ok {
		// Development mode: create in-memory registry and attestor
		registry = devFactory.CreateRegistry()
		attestor = devFactory.CreateAttestor()
	}

	// Step 3: Initialize trust domain parser (abstracts SDK trust domain validation)
	trustDomainParser := factory.CreateTrustDomainParser()

	// Step 4: Initialize identity credential parser (abstracts SDK parsing logic)
	parser := factory.CreateIdentityCredentialParser()

	// Step 5: Initialize identity document provider (abstracts SDK document creation/validation)
	docProvider := factory.CreateIdentityDocumentProvider()

	// Step 6: Initialize SPIRE server
	server, err := factory.CreateServer(ctx, config.TrustDomain, trustDomainParser, docProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE server: %w", err)
	}

	// Step 7-10: SEED registry and attestor (only in development mode)
	if registry != nil && attestor != nil {
		// Check if factory provides configurator capabilities
		registryConfigurator, hasRegistryConfig := factory.(ports.RegistryConfigurator)
		attestorConfigurator, hasAttestorConfig := factory.(ports.AttestorConfigurator)

		if hasAttestorConfig {
			// Step 7: SEED workload UID mappings (configuration, not runtime)
			for _, workload := range config.Workloads {
				attestorConfigurator.RegisterWorkloadUID(attestor, workload.UID, workload.Selector)
			}
		}

		if hasRegistryConfig {
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
				if err := registryConfigurator.SeedRegistry(registry, ctx, mapper); err != nil {
					return nil, fmt.Errorf("failed to seed registry for %s: %w", workload.SpiffeID, err)
				}
			}

			// Step 9: SEAL registry (prevent further mutations after seeding)
			registryConfigurator.SealRegistry(registry)
		}
	}

	// Step 10: Initialize SPIRE agent with registry
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
