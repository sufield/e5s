//go:build dev

package inmemory

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryTrustBundleProvider provides trust bundles for X.509 chain validation
// Aligns with go-spiffe bundle/x509bundle.Bundle: PEM-encoded multi-CA
// In skeleton: Returns provided CAs as PEM; production: bundle.Source
type InMemoryTrustBundleProvider struct {
	caCerts []*x509.Certificate
}

// NewInMemoryTrustBundleProvider creates a new in-memory trust bundle provider
// Supports multi-CA for bundle alignment with go-spiffe SDK
func NewInMemoryTrustBundleProvider(caCerts []*x509.Certificate) ports.TrustBundleProvider {
	return &InMemoryTrustBundleProvider{
		caCerts: caCerts,
	}
}

// GetBundle returns the trust bundle (PEM-encoded CA certs) for a trust domain
// Returns concatenated PEM blocks (multi-CA) matching go-spiffe bundle format
func (p *InMemoryTrustBundleProvider) GetBundle(ctx context.Context, trustDomain *domain.TrustDomain) ([]byte, error) {
	if trustDomain == nil {
		return nil, fmt.Errorf("inmemory: %w: trust domain cannot be nil", domain.ErrInvalidTrustDomain)
	}

	if len(p.caCerts) == 0 {
		return nil, fmt.Errorf("inmemory: %w: for trust domain %s", domain.ErrTrustBundleNotFound, trustDomain.String())
	}

	// Encode multi-CA as concatenated PEM blocks (matches SDK bundle format)
	// This allows seamless swap to real bundle.Source in production
	var bundlePEM []byte
	for _, cert := range p.caCerts {
		pemBlock := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		}
		bundlePEM = append(bundlePEM, pem.EncodeToMemory(pemBlock)...)
	}

	return bundlePEM, nil
}

// GetBundleForIdentity returns the trust bundle for an identity's trust domain
func (p *InMemoryTrustBundleProvider) GetBundleForIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) ([]byte, error) {
	if identityCredential == nil {
		return nil, fmt.Errorf("inmemory: %w: identity credential cannot be nil", domain.ErrInvalidIdentityCredential)
	}

	return p.GetBundle(ctx, identityCredential.TrustDomain())
}

var _ ports.TrustBundleProvider = (*InMemoryTrustBundleProvider)(nil)
