//go:build !dev

package app

import (
	"fmt"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// Application is the production composition root.
// Production version doesn't include Registry or demo Service - workloads only fetch identities via Workload API.
type Application struct {
	cfg   *ports.Config
	ics   ports.IdentityIssuer // server-side issuance facade
	agent ports.Agent
}

// New wires the production application and validates required deps.
func New(
	cfg *ports.Config,
	ics ports.IdentityIssuer,
	agent ports.Agent,
) (*Application, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if ics == nil {
		return nil, fmt.Errorf("identity issuer is nil")
	}
	if agent == nil {
		return nil, fmt.Errorf("agent is nil")
	}
	return &Application{
		cfg:   cfg,
		ics:   ics,
		agent: agent,
	}, nil
}

// Close releases resources owned by the application.
func (a *Application) Close() error {
	if a == nil || a.agent == nil {
		return nil
	}
	return a.agent.Close()
}

// Accessors (kept simple; add only what you actually need).
func (a *Application) Config() *ports.Config                { return a.cfg }
func (a *Application) IdentityIssuer() ports.IdentityIssuer { return a.ics }
func (a *Application) Agent() ports.Agent                   { return a.agent }
