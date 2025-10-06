package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/ports"
)

// IdentityClientService implements the server-side SVID issuance logic
// This is NOT the client interface - it's the internal service used by the server adapter
// The server adapter extracts credentials and calls this service
type IdentityClientService struct {
	agent ports.Agent
}

// NewIdentityClientService creates a new identity client service
func NewIdentityClientService(agent ports.Agent) *IdentityClientService {
	return &IdentityClientService{
		agent: agent,
	}
}

// FetchX509SVIDForCaller issues an SVID for a caller after credential extraction
// This is called by the server adapter (not by workloads directly)
// The adapter extracts callerIdentity from Unix socket connection before calling this
func (s *IdentityClientService) FetchX509SVIDForCaller(ctx context.Context, callerIdentity ports.ProcessIdentity) (*ports.Identity, error) {
	// Delegate to agent for attestation → matching → issuance
	identity, err := s.agent.FetchIdentityDocument(ctx, callerIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch identity document: %w", err)
	}

	return identity, nil
}

var _ ports.IdentityClientService = (*IdentityClientService)(nil)
