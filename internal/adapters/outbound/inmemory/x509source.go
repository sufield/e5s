//go:build dev

package inmemory

import (
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/dto"
)

// InMemoryX509Source is an in-memory implementation of x509svid.Source and x509bundle.Source
// It adapts domain IdentityDocument to go-spiffe SDK types
//
// Performance Note: Caches the parsed SPIFFE ID to avoid reparsing on every GetX509SVID() call.
// Useful in tight test loops where the source is queried frequently.
type InMemoryX509Source struct {
	identity    *dto.Identity
	trustDomain *domain.TrustDomain
	caBundle    []*x509.Certificate
	idCache     *spiffeid.ID // Cached parsed SPIFFE ID (set on first GetX509SVID call)
}

// NewInMemoryX509Source creates a new in-memory X509Source
//
// Defensive copy: Makes a defensive copy of caBundle to prevent external mutations.
// This is critical for test isolation - prevents one test from mutating another's bundle.
func NewInMemoryX509Source(
	identity *dto.Identity,
	trustDomain *domain.TrustDomain,
	caBundle []*x509.Certificate,
) (*InMemoryX509Source, error) {
	if identity == nil {
		return nil, fmt.Errorf("inmemory: identity is required")
	}
	if identity.IdentityDocument == nil {
		return nil, fmt.Errorf("inmemory: identity document is required")
	}
	if trustDomain == nil {
		return nil, fmt.Errorf("inmemory: trust domain is required")
	}
	if len(caBundle) == 0 {
		return nil, fmt.Errorf("inmemory: CA bundle is required")
	}

	// Defensive copy to avoid external mutation
	bundle := make([]*x509.Certificate, len(caBundle))
	copy(bundle, caBundle)

	return &InMemoryX509Source{
		identity:    identity,
		trustDomain: trustDomain,
		caBundle:    bundle,
	}, nil
}

// GetX509SVID implements x509svid.Source
//
// Performance: Caches the parsed SPIFFE ID on first call to avoid reparsing
// in tight test loops. Guards against nil/empty chain entries from fakes.
func (s *InMemoryX509Source) GetX509SVID() (*x509svid.SVID, error) {
	doc := s.identity.IdentityDocument

	// Parse SPIFFE ID from identity credential (cached after first parse)
	var spiffeID spiffeid.ID
	if s.idCache != nil {
		spiffeID = *s.idCache
	} else {
		parsed, err := spiffeid.FromString(s.identity.IdentityCredential.String())
		if err != nil {
			return nil, fmt.Errorf("inmemory: invalid SPIFFE ID: %w", err)
		}
		s.idCache = &parsed
		spiffeID = parsed
	}

	// Get leaf certificate
	leaf := doc.Certificate()
	if leaf == nil {
		return nil, fmt.Errorf("inmemory: missing leaf certificate")
	}

	// Build certificates array: [leaf cert, ...chain], filtering nils
	chain := doc.Chain()
	certificates := make([]*x509.Certificate, 1, 1+len(chain))
	certificates[0] = leaf
	for _, c := range chain {
		if c != nil {
			certificates = append(certificates, c)
		}
	}

	// Get private key from DTO (keys are managed outside the domain model)
	privateKey := s.identity.PrivateKey
	if privateKey == nil {
		return nil, fmt.Errorf("inmemory: missing private key in identity DTO")
	}

	return &x509svid.SVID{
		ID:           spiffeID,
		Certificates: certificates,
		PrivateKey:   privateKey,
	}, nil
}

// GetX509BundleForTrustDomain implements x509bundle.Source
//
// Trust domain comparison is case-insensitive (DNS-like). Returns defensive
// copy of CA bundle to prevent external mutation.
func (s *InMemoryX509Source) GetX509BundleForTrustDomain(trustDomain spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	// Check if requested trust domain matches ours (case-insensitive)
	// Trust domains are DNS-like and should be compared case-insensitively
	if !strings.EqualFold(trustDomain.String(), s.trustDomain.String()) {
		return nil, fmt.Errorf("inmemory: trust domain %s not found (only %s is available)", trustDomain, s.trustDomain)
	}

	// Copy CA bundle to avoid external mutation
	authorities := make([]*x509.Certificate, len(s.caBundle))
	copy(authorities, s.caBundle)

	// Create bundle from CA certificates
	return x509bundle.FromX509Authorities(trustDomain, authorities), nil
}

// TrustDomain returns the trust domain for this source (useful for tests)
func (s *InMemoryX509Source) TrustDomain() *domain.TrustDomain {
	return s.trustDomain
}

// Verify that InMemoryX509Source implements both interfaces
var _ x509svid.Source = (*InMemoryX509Source)(nil)
var _ x509bundle.Source = (*InMemoryX509Source)(nil)
