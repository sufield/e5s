//go:build dev

package inmemory

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Deterministic constants for fake - dev only
var (
	fakeTime   = time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	fakeSerial = big.NewInt(1)
)

// InMemoryServer is a deterministic fake SPIRE server (dev-only)
type InMemoryServer struct {
	trustDomain         *domain.TrustDomain
	caCert              *x509.Certificate
	caKey               *rsa.PrivateKey
	certificateProvider ports.IdentityDocumentProvider
}

// NewInMemoryServer creates a new in-memory SPIRE server
func NewInMemoryServer(ctx context.Context, trustDomainStr string, trustDomainParser ports.TrustDomainParser, certProvider ports.IdentityDocumentProvider) (*InMemoryServer, error) {
	if trustDomainParser == nil || certProvider == nil {
		return nil, fmt.Errorf("inmemory: trustDomainParser and certProvider are required")
	}

	// Use TrustDomainParser port to validate and create trust domain
	trustDomain, err := trustDomainParser.FromString(ctx, trustDomainStr)
	if err != nil {
		return nil, fmt.Errorf("invalid trust domain: %w", err)
	}

	// Generate CA certificate
	caCert, caKey, err := generateCA(trustDomainStr)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA: %w", err)
	}

	return &InMemoryServer{
		trustDomain:         trustDomain,
		caCert:              caCert,
		caKey:               caKey,
		certificateProvider: certProvider,
	}, nil
}

// IssueIdentity issues an X.509 identity document for an identity credential
func (s *InMemoryServer) IssueIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.IdentityDocument, error) {
	if identityCredential == nil {
		return nil, fmt.Errorf("inmemory: %w: identity credential cannot be nil", domain.ErrIdentityDocumentInvalid)
	}

	// Enforce trust-domain parity (credential must match server's trust domain)
	if identityCredential.TrustDomain().String() != s.trustDomain.String() {
		return nil, fmt.Errorf("inmemory: %w: trust domain mismatch: server=%s credential=%s",
			domain.ErrIdentityDocumentInvalid, s.trustDomain, identityCredential.TrustDomain())
	}

	if s.caCert == nil || s.caKey == nil {
		return nil, fmt.Errorf("inmemory: %w: CA not initialized", domain.ErrCANotInitialized)
	}

	doc, err := s.certificateProvider.CreateX509IdentityDocument(ctx, identityCredential, s.caCert, s.caKey)
	if err != nil {
		return nil, fmt.Errorf("inmemory: %w: %w", domain.ErrServerUnavailable, err)
	}

	return doc, nil
}

// GetTrustDomain returns the trust domain
func (s *InMemoryServer) GetTrustDomain() *domain.TrustDomain {
	return s.trustDomain
}

// GetCA returns the CA certificate (for agent initialization)
func (s *InMemoryServer) GetCA() *x509.Certificate {
	return s.caCert
}

// GetCABundle returns the CA bundle (single-root in dev)
func (s *InMemoryServer) GetCABundle() []*x509.Certificate {
	return []*x509.Certificate{s.caCert}
}

// generateCA creates a deterministic CA certificate for testing
func generateCA(trustDomain string) (*x509.Certificate, *rsa.PrivateKey, error) {
	// Use fixed test key (1024-bit minimum for Go crypto) - acceptable for fake/dev
	// Explicit PRNG seed for deterministic key generation
	privateKey, err := rsa.GenerateKey(&deterministicReader{state: 12345}, 1024)
	if err != nil {
		return nil, nil, fmt.Errorf("inmemory: key gen failed: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: fakeSerial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("Fake SPIRE CA %s", trustDomain),
		},
		NotBefore:             fakeTime,
		NotAfter:              fakeTime.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
		MaxPathLen:            1,
	}

	// Explicit PRNG seed for deterministic certificate generation
	certDER, err := x509.CreateCertificate(&deterministicReader{state: 67890}, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("inmemory: failed to create CA: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, fmt.Errorf("inmemory: failed to parse CA: %w", err)
	}

	return cert, privateKey, nil
}

// deterministicReader provides deterministic "random" bytes for testing
// Uses a simple LCG (Linear Congruential Generator) to provide deterministic but varied bytes
type deterministicReader struct {
	state uint64
}

func (r *deterministicReader) Read(p []byte) (n int, err error) {
	// LCG parameters (same as used in glibc)
	const (
		a = 1103515245
		c = 12345
		m = 1 << 31
	)

	for i := range p {
		r.state = (a*r.state + c) % m
		p[i] = byte(r.state >> 16)
	}
	return len(p), nil
}

var _ ports.IdentityServer = (*InMemoryServer)(nil)
