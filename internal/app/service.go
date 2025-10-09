package app

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// IdentityService implements identity-based message exchange
// Pure core logic: No HTTP, no TLS, no network here
// Dependencies are injected via ports (hexagonal architecture)
type IdentityService struct {
	agent    ports.Agent
	registry ports.IdentityMapperRegistry
}

// NewIdentityService creates a new identity-based service
func NewIdentityService(agent ports.Agent, registry ports.IdentityMapperRegistry) *IdentityService {
	return &IdentityService{
		agent:    agent,
		registry: registry,
	}
}

// ExchangeMessage performs authenticated message exchange
// This demonstrates the core business logic using identities
func (s *IdentityService) ExchangeMessage(ctx context.Context, from ports.Identity, to ports.Identity, content string) (*ports.Message, error) {
	// Core business logic: validate identities
	if from.IdentityCredential == nil {
		return nil, domain.ErrInvalidIdentityCredential
	}
	if to.IdentityCredential == nil {
		return nil, domain.ErrInvalidIdentityCredential
	}

	// Verify identity documents are valid
	if from.IdentityDocument == nil || !from.IdentityDocument.IsValid() {
		return nil, domain.ErrIdentityDocumentExpired
	}
	if to.IdentityDocument == nil || !to.IdentityDocument.IsValid() {
		return nil, domain.ErrIdentityDocumentExpired
	}

	// Create authenticated message
	msg := &ports.Message{
		From:    from,
		To:      to,
		Content: content,
	}

	return msg, nil
}

var _ ports.Service = (*IdentityService)(nil)
