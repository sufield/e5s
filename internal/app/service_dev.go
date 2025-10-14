//go:build dev

package app

import (
	"github.com/pocket/hexagon/spire/internal/ports"
)

// IdentityService implements identity-based message exchange for development.
// Development version includes registry for local identity mapping.
// Pure core logic: No HTTP, no TLS, no network here.
// Dependencies are injected via ports (hexagonal architecture).
type IdentityService struct {
	agent    ports.Agent
	registry ports.IdentityMapperRegistry
}

// NewIdentityService creates a new identity service for development.
func NewIdentityService(agent ports.Agent, registry ports.IdentityMapperRegistry) *IdentityService {
	return &IdentityService{
		agent:    agent,
		registry: registry,
	}
}

var _ ports.Service = (*IdentityService)(nil)
