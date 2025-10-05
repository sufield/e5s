package inmemory

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// InMemoryServer is an in-memory implementation of SPIRE server
type InMemoryServer struct {
	trustDomain          *domain.TrustDomain
	caCert               *x509.Certificate
	caKey                *rsa.PrivateKey
	certificateProvider  ports.IdentityDocumentProvider
	mu                   sync.RWMutex
}

// NewInMemoryServer creates a new in-memory SPIRE server
func NewInMemoryServer(ctx context.Context, trustDomainStr string, trustDomainParser ports.TrustDomainParser, certProvider ports.IdentityDocumentProvider) (*InMemoryServer, error) {
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

// IssueIdentity issues an X.509 identity document for an identity namespace
// No verification of registration - that's done by the agent during attestation/matching
func (s *InMemoryServer) IssueIdentity(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*domain.IdentityDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate inputs
	if identityNamespace == nil {
		return nil, fmt.Errorf("%w: identity namespace cannot be nil", domain.ErrIdentityDocumentInvalid)
	}

	if s.caCert == nil || s.caKey == nil {
		return nil, fmt.Errorf("%w: CA certificate or key not initialized", domain.ErrCANotInitialized)
	}

	// Use IdentityDocumentProvider port to create identity certificate (delegates certificate generation)
	doc, err := s.certificateProvider.CreateX509IdentityDocument(ctx, identityNamespace, s.caCert, s.caKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrServerUnavailable, err)
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

func generateCA(trustDomain string) (*x509.Certificate, *rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("SPIRE CA for %s", trustDomain),
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
		MaxPathLen:            1,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, err
	}

	return cert, privateKey, nil
}
var _ ports.Server = (*InMemoryServer)(nil)
