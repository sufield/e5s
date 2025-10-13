//go:build dev

package inmemory

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryIdentityCredentialParser implements the IdentityCredentialParser port for in-memory walking skeleton.
// Dev-only parser—production uses go-spiffe SDK (spiffeid.FromString/FromPath).
type InMemoryIdentityCredentialParser struct{}

// NewInMemoryIdentityCredentialParser creates a new in-memory identity credential parser
func NewInMemoryIdentityCredentialParser() ports.IdentityCredentialParser {
	return &InMemoryIdentityCredentialParser{}
}

// ParseFromString parses an identity credential from a URI string
// Format: spiffe://<trust-domain>/<path>
// Example: spiffe://example.org/host
func (p *InMemoryIdentityCredentialParser) ParseFromString(ctx context.Context, id string) (*domain.IdentityCredential, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("inmemory: %w: identity credential cannot be empty", domain.ErrInvalidIdentityCredential)
	}

	u, err := url.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("inmemory: %w: invalid URI format: %v", domain.ErrInvalidIdentityCredential, err)
	}
	if u.Scheme != "spiffe" {
		return nil, fmt.Errorf("inmemory: %w: must use 'spiffe' scheme, got: %s", domain.ErrInvalidIdentityCredential, u.Scheme)
	}
	if u.User != nil || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("inmemory: %w: URI must not include userinfo, port, query, or fragment", domain.ErrInvalidIdentityCredential)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("inmemory: %w: must contain a trust domain", domain.ErrInvalidIdentityCredential)
	}

	// Normalize trust domain to lowercase (SPIFFE trust domains are DNS-like)
	tdName := strings.ToLower(u.Host)
	trustDomain := domain.NewTrustDomainFromName(tdName)

	path := u.Path
	if path == "" {
		path = "/"
	} else if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return domain.NewIdentityCredentialFromComponents(trustDomain, path), nil
}

// ParseFromPath creates an identity credential from trust domain and path components
func (p *InMemoryIdentityCredentialParser) ParseFromPath(ctx context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityCredential, error) {
	if trustDomain == nil {
		return nil, fmt.Errorf("inmemory: %w: trust domain cannot be nil", domain.ErrInvalidIdentityCredential)
	}

	if path == "" {
		path = "/"
	} else if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return domain.NewIdentityCredentialFromComponents(trustDomain, path), nil
}

var _ ports.IdentityCredentialParser = (*InMemoryIdentityCredentialParser)(nil)
