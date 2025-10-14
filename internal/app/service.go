//go:build dev

package app

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// NOTE: IdentityService struct definition has been split into:
// - service_prod.go (//go:build !dev) - Production version with only agent field
// - service_dev.go (//go:build dev) - Development version with agent and registry fields
//
// This file contains ExchangeMessage demo method (dev-only).

// ExchangeMessage performs authenticated message exchange.
// This demonstrates the core business logic using identities.
// This method only uses the agent field, so it works in both prod and dev.
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
