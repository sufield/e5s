package inmemory

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// InMemoryIdentityNamespaceParser implements the IdentityNamespaceParser port for in-memory walking skeleton
// This provides simple string-based parsing without SDK dependencies.
// For a real implementation, this would use go-spiffe SDK's spiffeid.FromString/FromPath.
type InMemoryIdentityNamespaceParser struct{}

// NewInMemoryIdentityNamespaceParser creates a new in-memory identity namespace parser
func NewInMemoryIdentityNamespaceParser() ports.IdentityNamespaceParser {
	return &InMemoryIdentityNamespaceParser{}
}

// ParseFromString parses an identity namespace from a URI string
// Format: spiffe://<trust-domain>/<path>
// Example: spiffe://example.org/host
func (p *InMemoryIdentityNamespaceParser) ParseFromString(ctx context.Context, id string) (*domain.IdentityNamespace, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: identity namespace cannot be empty", domain.ErrInvalidIdentityNamespace)
	}

	// Parse as URI
	u, err := url.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid URI format: %v", domain.ErrInvalidIdentityNamespace, err)
	}

	// Validate scheme
	if u.Scheme != "spiffe" {
		return nil, fmt.Errorf("%w: must use 'spiffe' scheme, got: %s", domain.ErrInvalidIdentityNamespace, u.Scheme)
	}

	// Extract trust domain
	if u.Host == "" {
		return nil, fmt.Errorf("%w: must contain a trust domain", domain.ErrInvalidIdentityNamespace)
	}

	// Create trust domain from validated host (already checked for non-empty)
	trustDomain := domain.NewTrustDomainFromName(u.Host)

	// Extract path (default to "/" if empty)
	path := u.Path
	if path == "" {
		path = "/"
	}

	// Create domain IdentityNamespace from validated components
	return domain.NewIdentityNamespaceFromComponents(trustDomain, path), nil
}

// ParseFromPath creates an identity namespace from trust domain and path components
func (p *InMemoryIdentityNamespaceParser) ParseFromPath(ctx context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityNamespace, error) {
	if trustDomain == nil {
		return nil, fmt.Errorf("%w: trust domain cannot be nil", domain.ErrInvalidIdentityNamespace)
	}

	// Ensure path starts with "/"
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Create domain IdentityNamespace from components
	return domain.NewIdentityNamespaceFromComponents(trustDomain, path), nil
}
