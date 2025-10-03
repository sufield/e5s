package inmemory

import (
	"context"
	"fmt"
	"strings"

	"github.com/pocket/hexagon/spire/internal/app/ports"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// InMemoryTrustDomainParser implements the TrustDomainParser port for in-memory walking skeleton
// This provides simple string-based validation without SDK dependencies.
// For a real implementation, this would use go-spiffe SDK's spiffeid.TrustDomainFromString.
type InMemoryTrustDomainParser struct{}

// NewInMemoryTrustDomainParser creates a new in-memory trust domain parser
func NewInMemoryTrustDomainParser() ports.TrustDomainParser {
	return &InMemoryTrustDomainParser{}
}

// FromString parses a trust domain from a string
// Validates basic DNS format, ensures no scheme or path
func (p *InMemoryTrustDomainParser) FromString(ctx context.Context, name string) (*domain.TrustDomain, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: trust domain name cannot be empty", domain.ErrInvalidTrustDomain)
	}

	// Validate no scheme or path (trust domain should be just hostname)
	if strings.Contains(name, "://") {
		return nil, fmt.Errorf("%w: trust domain must not contain scheme", domain.ErrInvalidTrustDomain)
	}
	if strings.Contains(name, "/") {
		return nil, fmt.Errorf("%w: trust domain must not contain path", domain.ErrInvalidTrustDomain)
	}

	// In real implementation with SDK:
	// td, err := spiffeid.TrustDomainFromString(name)
	// if err != nil {
	//     return nil, fmt.Errorf("%w: %v", domain.ErrInvalidTrustDomain, err)
	// }
	// return domain.NewTrustDomainFromName(td.String()), nil

	// For walking skeleton: simple validation
	return domain.NewTrustDomainFromName(name), nil
}
