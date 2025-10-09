package inmemory

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryIdentityCredentialParser implements the IdentityCredentialParser port for in-memory walking skeleton
// This provides simple string-based parsing without SDK dependencies.
// For a real implementation, this would use go-spiffe SDK's spiffeid.FromString/FromPath.
type InMemoryIdentityCredentialParser struct{}

// NewInMemoryIdentityCredentialParser creates a new in-memory identity credential parser
func NewInMemoryIdentityCredentialParser() ports.IdentityCredentialParser {
	return &InMemoryIdentityCredentialParser{}
}

// ParseFromString parses an identity credential from a URI string
// Format: spiffe://<trust-domain>/<path>
// Example: spiffe://example.org/host
func (p *InMemoryIdentityCredentialParser) ParseFromString(ctx context.Context, id string) (*domain.IdentityCredential, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: identity credential cannot be empty", domain.ErrInvalidIdentityCredential)
	}

	// Parse as URI
	u, err := url.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid URI format: %v", domain.ErrInvalidIdentityCredential, err)
	}

	// Validate scheme
	if u.Scheme != "spiffe" {
		return nil, fmt.Errorf("%w: must use 'spiffe' scheme, got: %s", domain.ErrInvalidIdentityCredential, u.Scheme)
	}

	// Extract trust domain
	if u.Host == "" {
		return nil, fmt.Errorf("%w: must contain a trust domain", domain.ErrInvalidIdentityCredential)
	}

	// Create trust domain from validated host (already checked for non-empty)
	trustDomain := domain.NewTrustDomainFromName(u.Host)

	// Extract path (default to "/" if empty)
	path := u.Path
	if path == "" {
		path = "/"
	}

	// Create domain IdentityCredential from validated components
	return domain.NewIdentityCredentialFromComponents(trustDomain, path), nil
}

// ParseFromPath creates an identity credential from trust domain and path components
func (p *InMemoryIdentityCredentialParser) ParseFromPath(ctx context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityCredential, error) {
	if trustDomain == nil {
		return nil, fmt.Errorf("%w: trust domain cannot be nil", domain.ErrInvalidIdentityCredential)
	}

	// Ensure path starts with "/"
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Create domain IdentityCredential from components
	return domain.NewIdentityCredentialFromComponents(trustDomain, path), nil
}

var _ ports.IdentityCredentialParser = (*InMemoryIdentityCredentialParser)(nil)
