package spire

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// TrustDomainParser implements the TrustDomainParser port using go-spiffe SDK
// This provides production-grade trust domain parsing with proper DNS validation.
type TrustDomainParser struct{}

// NewTrustDomainParser creates a new SDK-based trust domain parser
func NewTrustDomainParser() ports.TrustDomainParser {
	return &TrustDomainParser{}
}

// FromString parses a trust domain from a string using go-spiffe SDK
// Uses spiffeid.TrustDomainFromString for proper validation:
// - DNS label format checking
// - No scheme or path allowed
// - Case-insensitive equality
func (p *TrustDomainParser) FromString(ctx context.Context, name string) (*domain.TrustDomain, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: trust domain name cannot be empty", domain.ErrInvalidTrustDomain)
	}

	// Use go-spiffe SDK for validation
	td, err := spiffeid.TrustDomainFromString(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidTrustDomain, err)
	}

	return domain.NewTrustDomainFromName(td.String()), nil
}

var _ ports.TrustDomainParser = (*TrustDomainParser)(nil)
