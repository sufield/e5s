//go:build !dev

package app

import (
	"fmt"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// Application wires production dependencies.
// In production there is no local registry or demo service;
// workloads fetch identities via the SPIRE Workload API through the Agent.
type Application struct {
	cfg   *ports.Config
	agent ports.Agent
}

// New constructs a production Application and validates required deps.
func New(cfg *ports.Config, agent ports.Agent) (*Application, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if agent == nil {
		return nil, fmt.Errorf("agent is nil")
	}
	return &Application{
		cfg:   cfg,
		agent: agent,
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
func (a *Application) Config() *ports.Config { return a.cfg }
func (a *Application) Agent() ports.Agent    { return a.agent }
