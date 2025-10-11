//go:build !dev

package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// Bootstrap creates and wires all application components for production mode.
// This version uses only CoreAdapterFactory and delegates to external SPIRE.
//
// Production mode:
// - Uses CoreAdapterFactory (minimal interface)
// - No in-memory registry or attestor (SPIRE Server/Agent handle these)
// - No seeding operations (SPIRE Server manages registration entries)
// - Agent connects to external SPIRE infrastructure
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, factory ports.CoreAdapterFactory) (*Application, error) {
	// Step 1: Load configuration
	config, err := configLoader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Step 2: Initialize trust domain parser (abstracts SDK trust domain validation)
	trustDomainParser := factory.CreateTrustDomainParser()

	// Step 3: Initialize identity credential parser (abstracts SDK parsing logic)
	parser := factory.CreateIdentityCredentialParser()

	// Step 4: Initialize identity document provider (abstracts SDK document creation/validation)
	docProvider := factory.CreateIdentityDocumentProvider()

	// Step 5: Initialize SPIRE server (connects to external SPIRE Server)
	// Note: In production, the server is optional - agents can work independently via Workload API
	server, err := factory.CreateServer(ctx, config.TrustDomain, trustDomainParser, docProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE server: %w", err)
	}
	_ = server // Server is created for potential future use but not currently needed by agent

	// Step 6: Initialize SPIRE agent (connects to external SPIRE Agent)
	// Production uses CreateProductionAgent with clean signature (only essential params)
	// Registry and attestor are handled by external SPIRE Server/Agent
	agent, err := factory.CreateProductionAgent(ctx, config.AgentSpiffeID, parser)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE agent: %w", err)
	}

	// Step 7: Initialize Identity Client service (server-side SVID issuance)
	identityClientService := NewIdentityClientService(agent)

	// Step 8: Initialize core service (demonstration use case)
	// In production: registry is nil (not used)
	service := NewIdentityService(agent, nil)

	return &Application{
		Config:                config,
		Service:               service,
		IdentityClientService: identityClientService,
		Agent:                 agent,
		Registry:              nil, // No registry in production (SPIRE Server handles this)
	}, nil
}
