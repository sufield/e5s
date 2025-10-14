//go:build !dev

package app

import (
	"github.com/pocket/hexagon/spire/internal/ports"
)

// IdentityService implements identity-based message exchange for production.
// Production version only needs agent - no local registry or attestation.
// Pure core logic: No HTTP, no TLS, no network here.
// Dependencies are injected via ports (hexagonal architecture).
type IdentityService struct {
	agent ports.Agent
}

// NewIdentityService creates a new identity service for production.
func NewIdentityService(agent ports.Agent) *IdentityService {
	return &IdentityService{
		agent: agent,
	}
}

var _ ports.Service = (*IdentityService)(nil)
