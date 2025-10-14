//go:build !dev

package app

import (
	"github.com/pocket/hexagon/spire/internal/ports"
)

// IdentityService provides identity operations for production.
// Production version only needs agent - no local registry, attestation, or demo service.
// In production, extend this with actual business logic (authorization, audit, etc.).
type IdentityService struct {
	agent ports.Agent
}

// NewIdentityService creates a new identity service for production.
func NewIdentityService(agent ports.Agent) *IdentityService {
	return &IdentityService{
		agent: agent,
	}
}
