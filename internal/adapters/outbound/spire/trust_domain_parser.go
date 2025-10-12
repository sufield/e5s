package spire

import (
	"context"
	"fmt"
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// TrustDomainParser implements the TrustDomainParser port using go-spiffe SDK.
// It returns a canonical, normalized trust domain string (lowercased, no scheme/path).
type TrustDomainParser struct{}

// NewTrustDomainParser creates a new SDK-based trust domain parser.
func NewTrustDomainParser() ports.TrustDomainParser {
	return &TrustDomainParser{}
}

// FromString parses and validates a trust domain using go-spiffe.
// Notes:
// - ctx is unused (kept for interface parity).
// - Input is trimmed; result is canonicalized by the SDK.
func (p *TrustDomainParser) FromString(_ context.Context, name string) (*domain.TrustDomain, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: trust domain name cannot be empty", domain.ErrInvalidTrustDomain)
	}

	td, err := spiffeid.TrustDomainFromString(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrInvalidTrustDomain, err)
	}

	return domain.NewTrustDomainFromName(td.String()), nil
}

var _ ports.TrustDomainParser = (*TrustDomainParser)(nil)
