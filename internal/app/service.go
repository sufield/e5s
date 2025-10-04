package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/app/ports"
)

// IdentityService implements identity-based message exchange
// Pure core logic: No HTTP, no TLS, no network here
// Dependencies are injected via ports (hexagonal architecture)
type IdentityService struct {
	agent ports.Agent
	store ports.IdentityStore
}

// NewIdentityService creates a new identity-based service
func NewIdentityService(agent ports.Agent, store ports.IdentityStore) *IdentityService {
	return &IdentityService{
		agent: agent,
		store: store,
	}
}

// ExchangeMessage performs authenticated message exchange
// This demonstrates the core business logic using identities
func (s *IdentityService) ExchangeMessage(ctx context.Context, from ports.Identity, to ports.Identity, content string) (*ports.Message, error) {
	// Core business logic: validate identities
	if from.IdentityNamespace == nil {
		return nil, fmt.Errorf("sender identity namespace required")
	}
	if to.IdentityNamespace == nil {
		return nil, fmt.Errorf("receiver identity namespace required")
	}

	// Verify identities exist in the store
	if _, err := s.store.GetIdentity(ctx, from.IdentityNamespace); err != nil {
		return nil, fmt.Errorf("sender identity verification failed: %w", err)
	}
	if _, err := s.store.GetIdentity(ctx, to.IdentityNamespace); err != nil {
		return nil, fmt.Errorf("receiver identity verification failed: %w", err)
	}

	// Verify identity documents are valid
	if from.IdentityDocument == nil || !from.IdentityDocument.IsValid() {
		return nil, fmt.Errorf("sender identity document invalid or expired")
	}
	if to.IdentityDocument == nil || !to.IdentityDocument.IsValid() {
		return nil, fmt.Errorf("receiver identity document invalid or expired")
	}

	// Create authenticated message
	msg := &ports.Message{
		From:    from,
		To:      to,
		Content: content,
	}

	return msg, nil
}

