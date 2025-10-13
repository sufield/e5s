//go:build dev

package inmemory

import (
	"bytes"
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
//
// Defensive copy: Filters out nil entries and non-CA certificates to avoid
// test setup errors. In dev mode, we skip non-CA certs rather than panic
// to allow flexible test configurations.
func NewInMemoryTrustBundleProvider(caCerts []*x509.Certificate) ports.TrustBundleProvider {
	// Defensive copy to avoid external mutation and filter invalid entries
	clone := make([]*x509.Certificate, 0, len(caCerts))
	for _, c := range caCerts {
		if c == nil {
			continue // Skip nil entries (common test gotcha)
		}
		// Dev-only: Ensure it's marked as a CA to match expectation of a trust anchor
		// We skip non-CAs rather than error to allow flexible test setups
		if !c.IsCA {
			continue // Skip non-CA certs
		}
		clone = append(clone, c)
	}
	return &InMemoryTrustBundleProvider{
		caCerts: clone,
	}
}

// GetBundle returns the trust bundle (PEM-encoded CA certs) for a trust domain
// Returns concatenated PEM blocks (multi-CA) matching go-spiffe bundle format
//
// Contract: Returns error if no bundle is available (consistent across dev components).
// Callers should not interpret empty bundles as success - always an error or valid bundle.
func (p *InMemoryTrustBundleProvider) GetBundle(ctx context.Context, trustDomain *domain.TrustDomain) ([]byte, error) {
	if trustDomain == nil {
		return nil, fmt.Errorf("inmemory: %w: trust domain cannot be nil", domain.ErrInvalidTrustDomain)
	}

	if len(p.caCerts) == 0 {
		return nil, fmt.Errorf("inmemory: %w: for trust domain %s", domain.ErrTrustBundleNotFound, trustDomain.String())
	}

	// Use buffer for efficient PEM assembly (fewer allocations than append)
	var buf bytes.Buffer
	for _, cert := range p.caCerts {
		if cert == nil {
			continue // Skip nil entries defensively (shouldn't happen after constructor filtering)
		}
		_ = pem.Encode(&buf, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
	}

	// Ensure we have at least one valid cert after filtering
	if buf.Len() == 0 {
		return nil, fmt.Errorf("inmemory: %w: for trust domain %s (no usable certs)", domain.ErrTrustBundleNotFound, trustDomain.String())
	}

	return buf.Bytes(), nil
}

// GetBundleForIdentity returns the trust bundle for an identity's trust domain
func (p *InMemoryTrustBundleProvider) GetBundleForIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) ([]byte, error) {
	if identityCredential == nil {
		return nil, fmt.Errorf("inmemory: %w: identity credential cannot be nil", domain.ErrInvalidIdentityCredential)
	}

	return p.GetBundle(ctx, identityCredential.TrustDomain())
}

var _ ports.TrustBundleProvider = (*InMemoryTrustBundleProvider)(nil)
