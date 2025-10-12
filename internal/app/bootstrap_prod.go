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

	// Step 2: Initialize parsers and providers (abstracts SDK logic)
	_ = factory.CreateTrustDomainParser()         // Created but unused - available for future expansion
	parser := factory.CreateIdentityCredentialParser()
	_ = factory.CreateIdentityDocumentProvider() // Created but unused - available for future expansion

	// NOTE: Production SPIRE workloads are clients only (via Workload API).
	// Real SPIRE Server runs as external infrastructure, not embedded in workload processes.
	// For development/testing with embedded server, use dev build with AdapterFactory instead.

	// Step 3: Initialize SPIRE agent (connects to external SPIRE Agent)
	// Production uses CreateProductionAgent with clean signature (only essential params)
	// Registry and attestor are handled by external SPIRE Server/Agent
	agent, err := factory.CreateProductionAgent(ctx, config.AgentSpiffeID, parser)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE agent: %w", err)
	}

	// Step 4: Initialize Identity Client service (server-side SVID issuance)
	identityClientService := NewIdentityClientService(agent)

	// Step 5: Initialize core service (demonstration use case)
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
