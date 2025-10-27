package app

import (
	"fmt"

	"github.com/pocket/hexagon/spire/internal/dto"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Application wires application dependencies.
//
// Identity Operations:
//   - Use IdentityService for SPIRE-agnostic identity operations (preferred)
//   - Agent provides lower-level SPIRE operations (use only when necessary)
//
// The IdentityService abstracts SPIRE implementation details and returns
// ports.Identity, making application code independent of SPIRE-specific types.
type Application struct {
	cfg             *dto.Config
	agent           ports.Agent
	identityService ports.IdentityService
}

// New constructs an Application and validates required deps.
func New(cfg *dto.Config, agent ports.Agent, identityService ports.IdentityService) (*Application, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if agent == nil {
		return nil, fmt.Errorf("agent is nil")
	}
	if identityService == nil {
		return nil, fmt.Errorf("identity service is nil")
	}
	return &Application{
		cfg:             cfg,
		agent:           agent,
		identityService: identityService,
	}, nil
}

// Close releases resources owned by the application (idempotent).
func (a *Application) Close() error {
	if a == nil || a.agent == nil {
		return nil
	}
	return a.agent.Close()
}

// Accessors (add only what you need)
func (a *Application) Config() *dto.Config                   { return a.cfg }
func (a *Application) Agent() ports.Agent                    { return a.agent }
func (a *Application) IdentityService() ports.IdentityService { return a.identityService }
