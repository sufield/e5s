//go:build dev

package app

import (
	"fmt"

	"github.com/pocket/hexagon/spire/internal/dto"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Application is the dev-mode composition root.
// Includes Registry for local identity mapping (dev-only).
type Application struct {
	cfg   *dto.Config
	svc   ports.Service        // optional demo service
	ics   ports.IdentityIssuer // server-side issuance facade
	agent ports.Agent
	reg   ports.IdentityMapperRegistry
}

// New wires the dev application and validates required deps.
func New(
	cfg *dto.Config,
	svc ports.Service,
	ics ports.IdentityIssuer,
	agent ports.Agent,
	reg ports.IdentityMapperRegistry,
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
	if reg == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	// svc is optional (can be nil for demo-less builds)
	return &Application{
		cfg:   cfg,
		svc:   svc,
		ics:   ics,
		agent: agent,
		reg:   reg,
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
func (a *Application) Config() *dto.Config                    { return a.cfg }
func (a *Application) Service() ports.Service                 { return a.svc }
func (a *Application) IdentityIssuer() ports.IdentityIssuer   { return a.ics }
func (a *Application) Agent() ports.Agent                     { return a.agent }
func (a *Application) Registry() ports.IdentityMapperRegistry { return a.reg }
