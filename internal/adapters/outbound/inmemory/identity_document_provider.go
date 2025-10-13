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
	"time"

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
	// Optional sanity (dev-only)
	if !caCertX509.IsCA {
		return nil, fmt.Errorf("inmemory: %w: provided CA cert is not a CA", domain.ErrIdentityDocumentInvalid)
	}

	// Deterministic-but-valid RSA key using a fixed PRNG
	prng := &deterministicReader{state: 54321}
	privateKey, err := rsa.GenerateKey(prng, 2048)
	if err != nil {
		return nil, fmt.Errorf("inmemory: %w: generate key: %w", domain.ErrIdentityDocumentInvalid, err)
	}

	identityURI, err := url.Parse(identityCredential.String())
	if err != nil {
		return nil, fmt.Errorf("inmemory: %w: invalid SPIFFE ID: %w", domain.ErrIdentityDocumentInvalid, err)
	}

	// 24h lifetime as intended
	notBefore := fakeTime
	notAfter := fakeTime.Add(24 * time.Hour)

	tmpl := &x509.Certificate{
		SerialNumber:          new(big.Int).Add(fakeSerial, big.NewInt(1)),
		Subject:               pkix.Name{CommonName: identityCredential.String()},
		URIs:                  []*url.URL{identityURI},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	certDER, err := x509.CreateCertificate(prng, tmpl, caCertX509, &privateKey.PublicKey, caKeyRSA)
	if err != nil {
		return nil, fmt.Errorf("inmemory: %w: create certificate: %w", domain.ErrIdentityDocumentInvalid, err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("inmemory: %w: parse certificate: %w", domain.ErrIdentityDocumentInvalid, err)
	}

	return domain.NewIdentityDocumentFromComponents(
		identityCredential,
		cert,
		privateKey,
		[]*x509.Certificate{cert, caCertX509},
	)
}

// ValidateIdentityDocument performs minimal validation (fake, dev-only)
func (p *InMemoryIdentityDocumentProvider) ValidateIdentityDocument(
	ctx context.Context,
	doc *domain.IdentityDocument,
	expectedID *domain.IdentityCredential,
) error {
	if doc == nil {
		return fmt.Errorf("inmemory: %w: nil document", domain.ErrIdentityDocumentInvalid)
	}
	if doc.IsExpired() {
		return domain.ErrIdentityDocumentExpired
	}
	if expectedID != nil && !doc.IdentityCredential().Equals(expectedID) {
		return fmt.Errorf("inmemory: %w: expected %s, got %s",
			domain.ErrIdentityDocumentMismatch, expectedID, doc.IdentityCredential())
	}
	return nil
}

var _ ports.IdentityDocumentProvider = (*InMemoryIdentityDocumentProvider)(nil)
