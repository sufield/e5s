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
	Config  *ports.Config
	Service ports.Service
	Agent   ports.Agent
	Store   ports.IdentityStore
}

// Bootstrap creates and wires all application components
// This is where dependency injection happens
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, deps ApplicationDeps) (*Application, error) {
	// Step 1: Load configuration
	config, err := configLoader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Step 2: Initialize identity store
	store := deps.CreateStore()

	// Step 3: Initialize trust domain parser (abstracts SDK trust domain validation)
	trustDomainParser := deps.CreateTrustDomainParser()

	// Step 4: Initialize identity format parser (abstracts SDK parsing logic)
	parser := deps.CreateIdentityNamespaceParser()

	// Step 5: Initialize identity document provider (abstracts SDK document creation/validation)
	docProvider := deps.CreateIdentityDocumentProvider()

	// Step 6: Initialize SPIRE server
	server, err := deps.CreateServer(ctx, config.TrustDomain, store, trustDomainParser, docProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE server: %w", err)
	}

	// Step 7: Initialize workload attestor
	attestor := deps.CreateAttestor()

	// Register workload UID mappings
	for _, workload := range config.Workloads {
		deps.RegisterWorkloadUID(attestor, workload.UID, workload.Selector)
	}

	// Step 8: Register workload identities
	for _, workload := range config.Workloads {
		// Use parser port instead of domain constructor
		identityFormat, err := parser.ParseFromString(ctx, workload.SpiffeID)
		if err != nil {
			return nil, fmt.Errorf("invalid identity format %s: %w", workload.SpiffeID, err)
		}
		selector, err := domain.ParseSelectorFromString(workload.Selector)
		if err != nil {
			return nil, fmt.Errorf("invalid selector %s: %w", workload.Selector, err)
		}
		if err := store.Register(ctx, identityFormat, selector); err != nil {
			return nil, fmt.Errorf("failed to register workload %s: %w", workload.SpiffeID, err)
		}
	}

	// Step 9: Initialize SPIRE agent (pass parser and document provider)
	agent, err := deps.CreateAgent(ctx, config.AgentSpiffeID, server, store, attestor, parser, docProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE agent: %w", err)
	}

	// Step 10: Initialize core service
	service := NewIdentityService(agent, store)

	return &Application{
		Config:  config,
		Service: service,
		Agent:   agent,
		Store:   store,
	}, nil
}

// ApplicationDeps abstracts the creation of concrete adapters
// This allows different implementations (in-memory, real SPIRE, etc.)
type ApplicationDeps interface {
	CreateStore() ports.IdentityStore
	CreateTrustDomainParser() ports.TrustDomainParser
	CreateIdentityNamespaceParser() ports.IdentityNamespaceParser
	CreateIdentityDocumentProvider() ports.IdentityDocumentProvider
	CreateServer(ctx context.Context, trustDomain string, store ports.IdentityStore, trustDomainParser ports.TrustDomainParser, docProvider ports.IdentityDocumentProvider) (ports.Server, error)
	CreateAttestor() ports.WorkloadAttestor
	RegisterWorkloadUID(attestor ports.WorkloadAttestor, uid int, selector string)
	CreateAgent(ctx context.Context, spiffeID string, server ports.Server, store ports.IdentityStore, attestor ports.WorkloadAttestor, parser ports.IdentityNamespaceParser, docProvider ports.IdentityDocumentProvider) (ports.Agent, error)
}
