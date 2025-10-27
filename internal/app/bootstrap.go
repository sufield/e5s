package app

import (
	"context"
	"fmt"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Bootstrap wires application components:
// - Loads config
// - Validates inputs
// - Applies a default timeout if the caller didn't set one
// - Builds an agent via SPIRE (Workload API)
// - Returns minimal Application for workloads
func Bootstrap(ctx context.Context, configLoader ports.ConfigLoader, factory *compose.SPIREAdapterFactory) (*Application, error) {
	if configLoader == nil {
		return nil, fmt.Errorf("config loader is nil")
	}
	if factory == nil {
		return nil, fmt.Errorf("SPIRE adapter factory is nil")
	}

	// Ensure we don't hang indefinitely if caller forgot a deadline
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
	}

	// Step 1: Load configuration
	cfg, err := configLoader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if cfg.AgentSpiffeID == "" {
		return nil, fmt.Errorf("agent SPIFFE ID is required")
	}

	// Step 2: Create identity credential parser (SDK-backed)
	credParser := factory.CreateIdentityCredentialParser()

	// Step 3: Create agent (connects to SPIRE via Workload API)
	agent, err := factory.CreateAgent(ctx, cfg.AgentSpiffeID, credParser)
	if err != nil {
		return nil, fmt.Errorf("create SPIRE agent: %w", err)
	}

	// Step 4: Wire application with constructor validation
	// Workloads only need Agent to fetch their own identity
	return New(cfg, agent)
}
