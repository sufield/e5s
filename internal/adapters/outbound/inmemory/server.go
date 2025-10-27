//go:build dev

// SECURITY NOTE:
// =============================================================================
// The CA in this file is for local development and testing ONLY.
//
// This CA is intentionally UNSAFE and PREDICTABLE:
//   - Keys and certificates are deterministic (same every run)
//   - NotBefore/NotAfter are fixed (year 2099)
//   - Serial numbers are predictable
//   - No cryptographic randomness
//   - No real security guarantees
//
// This design is for:
//   ✓ Reproducible tests (same certificates every run)
//   ✓ Fast local development (no key generation overhead)
//   ✓ Walking skeleton / demo mode
//
// NEVER use this CA, these keys, or these certificates in ANY production system.
// Production environments MUST use real SPIRE agents with proper attestation.
//
// This file cannot compile without the 'dev' build tag, which prevents
// accidental inclusion in production builds.
// =============================================================================

package inmemory

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Deterministic constants for fake CA - dev only
// WARNING: These are intentionally predictable and UNSAFE for production
var (
	fakeTime   = time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC) // Fixed timestamp for reproducibility
	fakeSerial = big.NewInt(1)                                // Predictable serial number
)

// InMemoryServer is a deterministic fake SPIRE server (dev-only)
// WARNING: Uses intentionally weak/predictable CA for testing. DO NOT USE IN PRODUCTION.
type InMemoryServer struct {
	trustDomain         *domain.TrustDomain
	caCert              *x509.Certificate
	caKey               *rsa.PrivateKey
	certificateProvider ports.IdentityDocumentProvider
}

// NewInMemoryServer creates a new in-memory SPIRE server with a deterministic CA.
//
// WARNING: This CA is intentionally weak and predictable for reproducible tests.
// DO NOT USE IN PRODUCTION BUILDS.
//
// The function enforces a build guard to prevent usage outside dev builds.
func NewInMemoryServer(ctx context.Context, trustDomainStr string, trustDomainParser ports.TrustDomainParser, certProvider ports.IdentityDocumentProvider) (*InMemoryServer, error) {
	// Defense-in-depth: Ensure this code only runs in dev builds
	// This should never fail (file has //go:build dev tag) but we check anyway
	if !devBuildGuard() {
		return nil, errors.New("inmemory: server cannot be used outside dev builds")
	}

	if trustDomainParser == nil || certProvider == nil {
		return nil, fmt.Errorf("inmemory: trustDomainParser and certProvider are required")
	}

	// Use TrustDomainParser port to validate and create trust domain
	trustDomain, err := trustDomainParser.FromString(ctx, trustDomainStr)
	if err != nil {
		return nil, fmt.Errorf("invalid trust domain: %w", err)
	}

	// Generate deterministic CA certificate (UNSAFE - for dev/test only)
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

// devBuildGuard returns true in dev builds.
// This file cannot compile without the 'dev' build tag, so this always returns true.
// The guard exists as defense-in-depth and to make the intent explicit in code.
func devBuildGuard() bool {
	return true
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
		return nil, fmt.Errorf("inmemory: %w: CA not initialized", ports.ErrCANotInitialized)
	}

	doc, err := s.certificateProvider.CreateX509IdentityDocument(ctx, identityCredential, s.caCert, s.caKey)
	if err != nil {
		return nil, fmt.Errorf("inmemory: %w: %w", ports.ErrServerUnavailable, err)
	}

	return doc, nil
}

// GetTrustDomain returns the trust domain
func (s *InMemoryServer) GetTrustDomain() *domain.TrustDomain {
	return s.trustDomain
}

// GetCA returns the CA certificate (for agent initialization)
// This is an internal helper, not part of ports.IdentityServer interface
func (s *InMemoryServer) GetCA() *x509.Certificate {
	return s.caCert
}

// GetCACertPEM returns the CA certificate as PEM bytes (required by ports.IdentityServer)
func (s *InMemoryServer) GetCACertPEM() []byte {
	if s.caCert == nil {
		return nil
	}
	// Convert x509.Certificate to PEM bytes
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: s.caCert.Raw,
	})
	return certPEM
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
