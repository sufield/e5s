package inmemory

import (
	"crypto"
	"crypto/x509"
	"fmt"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryX509Source is an in-memory implementation of x509svid.Source and x509bundle.Source
// It adapts domain IdentityDocument to go-spiffe SDK types
type InMemoryX509Source struct {
	identity    *ports.Identity
	trustDomain *domain.TrustDomain
	caBundle    []*x509.Certificate
}

// NewInMemoryX509Source creates a new in-memory X509Source
func NewInMemoryX509Source(
	identity *ports.Identity,
	trustDomain *domain.TrustDomain,
	caBundle []*x509.Certificate,
) (*InMemoryX509Source, error) {
	if identity == nil {
		return nil, fmt.Errorf("identity is required")
	}
	if identity.IdentityDocument == nil {
		return nil, fmt.Errorf("identity document is required")
	}
	if trustDomain == nil {
		return nil, fmt.Errorf("trust domain is required")
	}
	if len(caBundle) == 0 {
		return nil, fmt.Errorf("CA bundle is required")
	}

	return &InMemoryX509Source{
		identity:    identity,
		trustDomain: trustDomain,
		caBundle:    caBundle,
	}, nil
}

// GetX509SVID implements x509svid.Source
func (s *InMemoryX509Source) GetX509SVID() (*x509svid.SVID, error) {
	doc := s.identity.IdentityDocument

	// Parse SPIFFE ID from identity credential
	spiffeID, err := spiffeid.FromString(s.identity.IdentityCredential.String())
	if err != nil {
		return nil, fmt.Errorf("invalid SPIFFE ID: %w", err)
	}

	// Get certificate and chain
	cert := doc.Certificate()
	chain := doc.Chain()

	// Build certificates array: [leaf cert, ...chain]
	certificates := make([]*x509.Certificate, 0, 1+len(chain))
	certificates = append(certificates, cert)
	certificates = append(certificates, chain...)

	// Get private key
	privateKey := doc.PrivateKey()

	// Assert that private key implements crypto.Signer
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("private key does not implement crypto.Signer")
	}

	return &x509svid.SVID{
		ID:           spiffeID,
		Certificates: certificates,
		PrivateKey:   signer,
	}, nil
}

// GetX509BundleForTrustDomain implements x509bundle.Source
func (s *InMemoryX509Source) GetX509BundleForTrustDomain(trustDomain spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	// Check if requested trust domain matches ours
	if trustDomain.String() != s.trustDomain.String() {
		return nil, fmt.Errorf("trust domain %s not found (only %s is available)", trustDomain, s.trustDomain)
	}

	// Create bundle from CA certificates
	return x509bundle.FromX509Authorities(trustDomain, s.caBundle), nil
}

// Verify that InMemoryX509Source implements both interfaces
var _ x509svid.Source = (*InMemoryX509Source)(nil)
var _ x509bundle.Source = (*InMemoryX509Source)(nil)
