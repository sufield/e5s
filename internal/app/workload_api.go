//go:build dev

package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/app/identityconv"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/dto"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// IdentityClientService implements the server-side identity issuance logic (dev only).
// Server adapters extract workload credentials and call this service.
type IdentityClientService struct {
	agent ports.Agent
}

// NewIdentityClientService creates a new identity issuer service.
// Returns error if agent is nil (fail-fast validation).
func NewIdentityClientService(agent ports.Agent) (*IdentityClientService, error) {
	if agent == nil {
		return nil, fmt.Errorf("agent cannot be nil")
	}
	return &IdentityClientService{agent: agent}, nil
}

// IssueIdentity creates an identity credential for an authenticated workload.
// Flow (dev): validate workload → delegate to agent → adapt to dto.Identity.
//
// Error semantics:
//   - domain.ErrWorkloadInvalid: workload validation failed
//   - domain.ErrIdentityDocumentInvalid: nil/empty document returned
//   - wrapped agent errors: attestation/matching/issuance failures
func (s *IdentityClientService) IssueIdentity(
	ctx context.Context,
	workload *domain.Workload,
) (*dto.Identity, error) {
	// 1) Validate workload.
	if workload == nil {
		return nil, fmt.Errorf("%w: nil workload", domain.ErrWorkloadInvalid)
	}
	if err := workload.Validate(); err != nil {
		return nil, fmt.Errorf("validate workload: %w", err)
	}

	// 2) Delegate to agent for attestation → matching → issuance.
	doc, err := s.agent.FetchIdentityDocument(ctx, workload)
	if err != nil {
		return nil, fmt.Errorf("fetch identity document: %w", err)
	}
	if doc == nil || doc.IdentityCredential() == nil {
		return nil, fmt.Errorf("%w: empty identity document", domain.ErrIdentityDocumentInvalid)
	}

	// 3) Build DTO. Name is best-effort sugar for logs/UI.
	cred := doc.IdentityCredential()
	name := identityconv.DeriveIdentityName(cred)

	return &dto.Identity{
		IdentityCredential: cred,
		IdentityDocument:   doc,
		Name:               name,
	}, nil
}

var _ ports.IdentityIssuer = (*IdentityClientService)(nil)
