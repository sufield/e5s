package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// IdentityClientService implements the server-side identity issuance logic.
// This is the internal service used by server adapters to issue identity credentials.
// Server adapters extract workload credentials and call this service.
type IdentityClientService struct {
	agent ports.Agent
}

// NewIdentityClientService creates a new identity issuer service.
func NewIdentityClientService(agent ports.Agent) *IdentityClientService {
	return &IdentityClientService{
		agent: agent,
	}
}

// IssueIdentity creates an identity credential for an authenticated workload.
// This is called by server adapters after extracting the workload's credentials
// from a trusted source (e.g., Unix socket peer credentials).
func (s *IdentityClientService) IssueIdentity(ctx context.Context, workload ports.ProcessIdentity) (*ports.Identity, error) {
	// Delegate to agent for attestation → matching → issuance
	identity, err := s.agent.FetchIdentityDocument(ctx, workload)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch identity document: %w", err)
	}

	return identity, nil
}

var _ ports.IdentityIssuer = (*IdentityClientService)(nil)
