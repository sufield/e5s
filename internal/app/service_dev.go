//go:build dev

package app

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// IdentityService implements identity-based message exchange for development.
// Development version includes registry for local identity mapping.
// Pure core logic: No HTTP, no TLS, no network here.
// Dependencies are injected via ports (hexagonal architecture).
//
// NOTE: This is dev-only. Production builds do not include this service.
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

// ExchangeMessage is a dev-only demo API for authenticated message exchange.
// Validates identity credentials and documents before allowing message exchange.
//
// Error semantics:
//   - ErrInvalidIdentityCredential: nil credential
//   - ErrIdentityDocumentInvalid: nil document
//   - ErrIdentityDocumentExpired: expired/invalid document
func (s *IdentityService) ExchangeMessage(
	_ context.Context,
	from ports.Identity,
	to ports.Identity,
	content string,
) (*ports.Message, error) {
	// Validate identity credentials are present
	if from.IdentityCredential == nil || to.IdentityCredential == nil {
		return nil, domain.ErrInvalidIdentityCredential
	}

	// Validate identity documents are present (nil check)
	if from.IdentityDocument == nil {
		return nil, domain.ErrIdentityDocumentInvalid
	}
	if to.IdentityDocument == nil {
		return nil, domain.ErrIdentityDocumentInvalid
	}

	// Validate identity documents are valid (not expired)
	if !from.IdentityDocument.IsValid() {
		return nil, domain.ErrIdentityDocumentExpired
	}
	if !to.IdentityDocument.IsValid() {
		return nil, domain.ErrIdentityDocumentExpired
	}

	return &ports.Message{
		From:    from,
		To:      to,
		Content: content,
	}, nil
}

var _ ports.Service = (*IdentityService)(nil)
