package spire

import (
	"context"
	"fmt"
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// IdentityCredentialParser implements the IdentityCredentialParser port using go-spiffe SDK
// This provides production-grade identity credential parsing with proper validation.
type IdentityCredentialParser struct{}

// NewIdentityCredentialParser creates a new SDK-based identity credential parser
func NewIdentityCredentialParser() ports.IdentityCredentialParser {
	return &IdentityCredentialParser{}
}

// ParseFromString parses an identity credential from a URI string using go-spiffe SDK
// Uses spiffeid.FromString for proper validation:
// - Scheme validation (must be "spiffe")
// - Trust domain format checking
// - Path normalization
//
// Format: spiffe://<trust-domain>/<path>
// Example: spiffe://example.org/host
func (p *IdentityCredentialParser) ParseFromString(ctx context.Context, id string) (*domain.IdentityCredential, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: identity credential cannot be empty", domain.ErrInvalidIdentityCredential)
	}

	// Use go-spiffe SDK for validation
	spiffeID, err := spiffeid.FromString(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidIdentityCredential, err)
	}

	// Convert to domain IdentityCredential via translation helper
	return TranslateSPIFFEIDToIdentityCredential(spiffeID)
}

// ParseFromPath creates an identity credential from trust domain and path components
// Uses spiffeid.FromPath for proper path validation and normalization
func (p *IdentityCredentialParser) ParseFromPath(ctx context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityCredential, error) {
	if trustDomain == nil {
		return nil, fmt.Errorf("%w: trust domain cannot be nil", domain.ErrInvalidIdentityCredential)
	}

	// Convert domain TrustDomain to go-spiffe TrustDomain
	td, err := spiffeid.TrustDomainFromString(trustDomain.String())
	if err != nil {
		return nil, fmt.Errorf("%w: invalid trust domain: %v", domain.ErrInvalidIdentityCredential, err)
	}

	// Use go-spiffe SDK to create ID from components
	var spiffeID spiffeid.ID

	// Handle empty path (root SPIFFE ID)
	if path == "" || path == "/" {
		// For root path, use FromString with just trust domain
		spiffeID, err = spiffeid.FromString("spiffe://" + td.String())
	} else {
		// Ensure path starts with "/" for non-root paths
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		spiffeID, err = spiffeid.FromPath(td, path)
	}

	if err != nil {
		return nil, fmt.Errorf("%w: invalid path: %v", domain.ErrInvalidIdentityCredential, err)
	}

	// Convert to domain IdentityCredential via translation helper
	return TranslateSPIFFEIDToIdentityCredential(spiffeID)
}

var _ ports.IdentityCredentialParser = (*IdentityCredentialParser)(nil)
