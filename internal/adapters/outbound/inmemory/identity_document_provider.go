//go:build dev

package inmemory

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net/url"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryIdentityDocumentProvider is a deterministic fake for X.509 SVID generation (dev-only)
type InMemoryIdentityDocumentProvider struct{}

// NewInMemoryIdentityDocumentProvider creates a new in-memory identity certificate provider
func NewInMemoryIdentityDocumentProvider() ports.IdentityDocumentProvider {
	return &InMemoryIdentityDocumentProvider{}
}

// CreateX509IdentityDocument creates a deterministic X.509 SVID (fake, dev-only)
func (p *InMemoryIdentityDocumentProvider) CreateX509IdentityDocument(
	ctx context.Context,
	identityCredential *domain.IdentityCredential,
	caCert interface{},
	caKey interface{},
) (*domain.IdentityDocument, error) {
	if identityCredential == nil {
		return nil, fmt.Errorf("inmemory: %w: identity credential cannot be nil", domain.ErrIdentityDocumentInvalid)
	}

	caCertX509, ok := caCert.(*x509.Certificate)
	if !ok {
		return nil, fmt.Errorf("inmemory: %w: CA certificate must be *x509.Certificate", domain.ErrIdentityDocumentInvalid)
	}

	caKeyRSA, ok := caKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("inmemory: %w: CA key must be *rsa.PrivateKey", domain.ErrIdentityDocumentInvalid)
	}

	// Deterministic key - acceptable for fake
	privateKey := &rsa.PrivateKey{
		D: big.NewInt(2),
		Primes: []*big.Int{
			big.NewInt(59), big.NewInt(47),
		},
	}
	privateKey.PublicKey = rsa.PublicKey{
		N: big.NewInt(2773),
		E: 65537,
	}
	privateKey.Precompute()

	identityURI, err := url.Parse(identityCredential.String())
	if err != nil {
		return nil, fmt.Errorf("inmemory: %w: invalid SPIFFE ID: %v", domain.ErrIdentityDocumentInvalid, err)
	}

	template := &x509.Certificate{
		SerialNumber: new(big.Int).Add(fakeSerial, big.NewInt(1)),
		Subject: pkix.Name{
			CommonName: identityCredential.String(),
		},
		URIs:                  []*url.URL{identityURI},
		NotBefore:             fakeTime,
		NotAfter:              fakeTime.Add(24 * 365 * 24 * 3600 * 1000000000), // 24h in nanoseconds
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(&deterministicReader{state: 54321}, template, caCertX509, &privateKey.PublicKey, caKeyRSA)
	if err != nil {
		return nil, fmt.Errorf("inmemory: %w: failed to create certificate: %v", domain.ErrIdentityDocumentInvalid, err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("inmemory: %w: failed to parse certificate: %v", domain.ErrIdentityDocumentInvalid, err)
	}

	return domain.NewIdentityDocumentFromComponents(
		identityCredential,
		cert,
		privateKey,
		[]*x509.Certificate{caCertX509},
		template.NotAfter,
	), nil
}

// ValidateIdentityDocument performs minimal validation (fake, dev-only)
func (p *InMemoryIdentityDocumentProvider) ValidateIdentityDocument(
	ctx context.Context,
	cert *domain.IdentityDocument,
	expectedID *domain.IdentityCredential,
) error {
	if cert == nil {
		return fmt.Errorf("inmemory: %w: nil document", domain.ErrIdentityDocumentInvalid)
	}

	if cert.IsExpired() {
		return domain.ErrIdentityDocumentExpired
	}

	if expectedID != nil && !cert.IdentityCredential().Equals(expectedID) {
		return fmt.Errorf("inmemory: %w: expected %s, got %s", domain.ErrIdentityDocumentMismatch, expectedID, cert.IdentityCredential())
	}

	return nil
}

var _ ports.IdentityDocumentProvider = (*InMemoryIdentityDocumentProvider)(nil)
