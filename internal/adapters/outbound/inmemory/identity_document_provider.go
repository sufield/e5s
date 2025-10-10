package inmemory

import (
	"context"
	"crypto/rand"
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

// InMemoryIdentityDocumentProvider implements the IdentityDocumentProvider port for in-memory walking skeleton
// This provides simple X.509 certificate generation without SDK dependencies.
// For a real implementation, this would use go-spiffe SDK's x509svid package.
type InMemoryIdentityDocumentProvider struct{}

// NewInMemoryIdentityDocumentProvider creates a new in-memory identity certificate provider
func NewInMemoryIdentityDocumentProvider() ports.IdentityDocumentProvider {
	return &InMemoryIdentityDocumentProvider{}
}

// CreateX509IdentityDocument creates an X.509 identity certificate by generating a certificate signed by the CA
func (p *InMemoryIdentityDocumentProvider) CreateX509IdentityDocument(
	ctx context.Context,
	identityCredential *domain.IdentityCredential,
	caCert interface{},
	caKey interface{},
) (*domain.IdentityDocument, error) {
	if identityCredential == nil {
		return nil, fmt.Errorf("%w: identity credential cannot be nil", domain.ErrIdentityDocumentInvalid)
	}

	// Type assert CA cert and key
	caCertX509, ok := caCert.(*x509.Certificate)
	if !ok {
		return nil, fmt.Errorf("%w: CA certificate must be *x509.Certificate", domain.ErrIdentityDocumentInvalid)
	}

	caKeyRSA, ok := caKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: CA key must be *rsa.PrivateKey", domain.ErrIdentityDocumentInvalid)
	}

	// Generate key pair for identity certificate
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to generate private key: %v", domain.ErrIdentityDocumentInvalid, err)
	}

	// Parse identity credential as URI for certificate
	identityURI, err := url.Parse(identityCredential.String())
	if err != nil {
		return nil, fmt.Errorf("%w: invalid identity credential URI: %v", domain.ErrIdentityDocumentInvalid, err)
	}

	// Create certificate template
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	notBefore := time.Now()
	notAfter := notBefore.Add(24 * time.Hour)

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: identityCredential.String(),
		},
		URIs:                  []*url.URL{identityURI},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Sign certificate with CA
	certDER, err := x509.CreateCertificate(rand.Reader, template, caCertX509, &privateKey.PublicKey, caKeyRSA)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create certificate: %v", domain.ErrIdentityDocumentInvalid, err)
	}

	// Parse DER to x509.Certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse certificate: %v", domain.ErrIdentityDocumentInvalid, err)
	}

	// Create domain identity certificate from validated components
	return domain.NewIdentityDocumentFromComponents(
		identityCredential,
		cert,
		privateKey,
		[]*x509.Certificate{caCertX509},
		notAfter,
	), nil
}

// ValidateIdentityDocument performs identity certificate validation
func (p *InMemoryIdentityDocumentProvider) ValidateIdentityDocument(
	ctx context.Context,
	cert *domain.IdentityDocument,
	expectedID *domain.IdentityCredential,
) error {
	if cert == nil {
		return fmt.Errorf("%w: identity certificate is nil", domain.ErrIdentityDocumentInvalid)
	}

	// Check expiration
	if cert.IsExpired() {
		return domain.ErrIdentityDocumentExpired
	}

	// Check identity credential match
	if expectedID != nil && !cert.IdentityCredential().Equals(expectedID) {
		return fmt.Errorf("%w: expected %s, got %s", domain.ErrIdentityDocumentMismatch, expectedID.String(), cert.IdentityCredential().String())
	}

	// Check X.509 certificate validity window
	x509Cert := cert.Certificate()
	if x509Cert == nil {
		return fmt.Errorf("%w: X.509 identity certificate missing certificate", domain.ErrIdentityDocumentInvalid)
	}

	now := time.Now()
	if now.Before(x509Cert.NotBefore) || now.After(x509Cert.NotAfter) {
		return domain.ErrIdentityDocumentExpired
	}

	// In real implementation with SDK:
	// - Verify certificate chain with trust bundle
	// - Check signature validity
	// - Validate identity credential in certificate URIs
	// Example: x509svid.Verify(cert, chain, bundle)

	return nil
}

var _ ports.IdentityDocumentProvider = (*InMemoryIdentityDocumentProvider)(nil)
