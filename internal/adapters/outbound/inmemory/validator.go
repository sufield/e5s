package inmemory

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// IdentityDocumentValidator validates identity certificates using standard x509 checks
// In a real implementation with go-spiffe SDK, this would use:
// - x509svid.ParseAndVerify for chain-of-trust validation
// - Bundle verification against trust domain
// - Full X.509 path validation
//
// For now, this provides basic validation without SDK dependencies
type IdentityDocumentValidator struct{}

// NewIdentityDocumentValidator creates a new identity certificate validator
func NewIdentityDocumentValidator() *IdentityDocumentValidator {
	return &IdentityDocumentValidator{}
}

// Validate checks if an identity certificate is valid for the given identity namespace
// This is the anti-corruption layer between domain and SDK validation logic
func (v *IdentityDocumentValidator) Validate(ctx context.Context, cert *domain.IdentityDocument, expectedID *domain.IdentityNamespace) error {
	if cert == nil {
		return fmt.Errorf("%w: identity document cannot be nil", domain.ErrIdentityDocumentInvalid)
	}

	if expectedID == nil {
		return fmt.Errorf("%w: expected identity namespace cannot be nil", domain.ErrInvalidIdentityNamespace)
	}

	// Check time validity (using domain method that wraps x509)
	if !cert.IsValid() {
		return fmt.Errorf("%w: certificate expired or not yet valid", domain.ErrIdentityDocumentExpired)
	}

	// Check identity namespace matches expected
	if !cert.IdentityNamespace().Equals(expectedID) {
		return fmt.Errorf("%w: expected %s, got %s",
			domain.ErrIdentityDocumentMismatch, expectedID.String(), cert.IdentityNamespace().String())
	}

	// In a real implementation with go-spiffe SDK, you would add:
	// 1. Chain-of-trust verification using x509svid.Verify(cert.Certificate(), cert.Chain(), bundle)
	// 2. Bundle-based trust domain validation
	// 3. Full X.509 path validation
	// 4. CRL/OCSP checks if needed
	//
	// Example with SDK:
	// bundle := ... // get trust bundle for trust domain
	// _, err := x509svid.Verify(cert.Certificate(), cert.Chain(), bundle)
	// if err != nil {
	//     return fmt.Errorf("identity certificate verification failed: %w", err)
	// }

	return nil
}

// Ensure IdentityDocumentValidator implements the port
var _ ports.IdentityDocumentValidator = (*IdentityDocumentValidator)(nil)
