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

// FromString parses a trust domain from a string with DNS-like validation.
// Notes:
// - ctx is unused (kept for interface parity).
// - Input is trimmed and lowercased for normalization.
// - Validates DNS-like structure (labels separated by dots).
// - Rejects invalid labels, illegal characters, and malformed input.
// - Length limits: labels ≤63 chars, total ≤253 chars (DNS standard).
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

	// Reject common bad inputs (fast checks)
	if strings.HasPrefix(name, ".") {
		return nil, fmt.Errorf("inmemory: %w: trust domain must not start with dot", domain.ErrInvalidTrustDomain)
	}
	if strings.HasSuffix(name, ".") {
		return nil, fmt.Errorf("inmemory: %w: trust domain must not end with dot", domain.ErrInvalidTrustDomain)
	}
	if strings.Contains(name, "..") {
		return nil, fmt.Errorf("inmemory: %w: trust domain must not contain consecutive dots", domain.ErrInvalidTrustDomain)
	}

	// Check total length (DNS limit: 253 chars)
	if len(name) > 253 {
		return nil, fmt.Errorf("inmemory: %w: trust domain exceeds 253 characters", domain.ErrInvalidTrustDomain)
	}

	// Validate characters (only [a-z0-9-.] allowed after lowercasing)
	if idx := strings.IndexFunc(name, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '.')
	}); idx != -1 {
		return nil, fmt.Errorf("inmemory: %w: trust domain contains illegal character at position %d", domain.ErrInvalidTrustDomain, idx)
	}

	// Validate labels (split by dot and check each label)
	labels := strings.Split(name, ".")
	for _, label := range labels {
		// Reject empty labels (e.g., "example..org" would have empty label)
		if label == "" {
			return nil, fmt.Errorf("inmemory: %w: trust domain contains empty label", domain.ErrInvalidTrustDomain)
		}

		// Reject labels starting or ending with hyphen
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return nil, fmt.Errorf("inmemory: %w: label %q must not start or end with hyphen", domain.ErrInvalidTrustDomain, label)
		}

		// Check label length (DNS limit: 63 chars per label)
		if len(label) > 63 {
			return nil, fmt.Errorf("inmemory: %w: label %q exceeds 63 characters", domain.ErrInvalidTrustDomain, label)
		}
	}

	return domain.NewTrustDomainFromName(name), nil
}

var _ ports.TrustDomainParser = (*InMemoryTrustDomainParser)(nil)
