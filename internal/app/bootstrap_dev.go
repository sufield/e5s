//go:build dev

package app

import (
	"context"
	"fmt"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Bootstrap creates and wires all application components for development mode.
//
// Development mode:
// - Uses concrete InMemoryAdapterFactory (no interfaces)
// - Components are pre-configured from config when created
// - Validates inputs up-front
// - Guards against infinite waits with default timeout
// - Best-effort cleanup on partial failures
// - Returns Application with Close() for tidy shutdown
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, factory *compose.InMemoryAdapterFactory) (*Application, error) {
	// Validate inputs up-front
	if configLoader == nil {
		return nil, fmt.Errorf("config loader is nil")
	}
	if factory == nil {
		return nil, fmt.Errorf("in-memory factory is nil")
	}

	// In dev, guard against accidental infinite waits
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}

	// Step 1: Load configuration
	cfg, err := configLoader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Step 2: Initialize parsers and providers
	tdParser := factory.CreateTrustDomainParser()
	idParser := factory.CreateIdentityCredentialParser()
	docProvider := factory.CreateIdentityDocumentProvider()

	// Step 3: Create server
	server, err := factory.CreateServer(ctx, cfg.TrustDomain, tdParser, docProvider)
	if err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}

	// Step 4: Create registry (seeded with workload configurations)
	registry, err := factory.CreateRegistry(ctx, cfg.Workloads, idParser)
	if err != nil {
		// Server has no Close() in dev; nothing to clean up
		return nil, fmt.Errorf("create registry: %w", err)
	}

	// Step 5: Create attestor (configured with workload UIDs)
	attestor := factory.CreateAttestor(cfg.Workloads)

	// Step 6: Create agent
	agent, err := factory.CreateAgent(ctx, cfg.AgentSpiffeID, server, registry, attestor, idParser, docProvider)
	if err != nil {
		// Server and registry have no Close() in dev; nothing to clean up
		return nil, fmt.Errorf("create agent: %w", err)
	}

	// Step 7: Initialize services
	identitySvc := NewIdentityClientService(agent)
	service := NewIdentityService(agent, registry)

	// Step 8: Wire application with constructor validation
	return New(cfg, service, identitySvc, agent, registry)
}
