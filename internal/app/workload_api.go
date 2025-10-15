//go:build dev

package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/app/identityconv"
	"github.com/pocket/hexagon/spire/internal/domain"
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
// Flow (dev): validate process identity → delegate to agent → adapt to ports.Identity.
//
// Error semantics:
//   - domain.ErrInvalidProcessIdentity: workload validation failed
//   - domain.ErrIdentityDocumentInvalid: nil/empty document returned
//   - wrapped agent errors: attestation/matching/issuance failures
func (s *IdentityClientService) IssueIdentity(
	ctx context.Context,
	workload ports.ProcessIdentity,
) (*ports.Identity, error) {
	// 1) Validate inputs early with dev helper (returns domain.ErrInvalidProcessIdentity).
	if err := identityconv.ValidateProcessIdentity(workload); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	// 2) Delegate to agent for attestation → matching → issuance.
	doc, err := s.agent.FetchIdentityDocument(ctx, workload)
	if err != nil {
		return nil, fmt.Errorf("fetch identity document: %w", err)
	}
	if doc == nil || doc.IdentityCredential() == nil {
		return nil, fmt.Errorf("%w: empty identity document", domain.ErrIdentityDocumentInvalid)
	}

	// 3) Build ports DTO. Name is best-effort sugar for logs/UI.
	cred := doc.IdentityCredential()
	name := identityconv.DeriveIdentityName(cred)

	return &ports.Identity{
		IdentityCredential: cred,
		IdentityDocument:   doc,
		Name:               name,
	}, nil
}

var _ ports.IdentityIssuer = (*IdentityClientService)(nil)
