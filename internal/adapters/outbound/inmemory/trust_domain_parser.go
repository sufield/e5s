//go:build dev

package inmemory

import (
	"context"
	"fmt"
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryTrustDomainParser implements the TrustDomainParser port for deterministic fake (dev-only).
// Returns a normalized trust domain string (lowercased).
// Provides simple validation without cryptographic dependencies.
type InMemoryTrustDomainParser struct{}

// NewInMemoryTrustDomainParser creates a new in-memory trust domain parser
func NewInMemoryTrustDomainParser() ports.TrustDomainParser {
	return &InMemoryTrustDomainParser{}
}

// FromString parses a trust domain from a string with basic validation.
// Notes:
// - ctx is unused (kept for interface parity).
// - Input is trimmed and lowercased for normalization.
// - Validates no scheme or path (trust domain should be just hostname).
func (p *InMemoryTrustDomainParser) FromString(_ context.Context, name string) (*domain.TrustDomain, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("inmemory: %w: trust domain name cannot be empty", domain.ErrInvalidTrustDomain)
	}

	// Validate no scheme or path (trust domain should be just hostname)
	if strings.Contains(name, "://") {
		return nil, fmt.Errorf("inmemory: %w: trust domain must not contain scheme", domain.ErrInvalidTrustDomain)
	}
	if strings.Contains(name, "/") {
		return nil, fmt.Errorf("inmemory: %w: trust domain must not contain path", domain.ErrInvalidTrustDomain)
	}

	// Normalize: lowercase for case-insensitive matching
	name = strings.ToLower(name)

	return domain.NewTrustDomainFromName(name), nil
}

var _ ports.TrustDomainParser = (*InMemoryTrustDomainParser)(nil)
