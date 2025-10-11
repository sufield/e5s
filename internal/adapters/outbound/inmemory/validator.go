package inmemory

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// IdentityDocumentValidator validates identity certificates using standard x509 checks
// In a real implementation with go-spiffe SDK, this would use:
// - x509svid.ParseAndVerify for chain-of-trust validation
// - Bundle verification against trust domain
// - Full X.509 path validation
//
// For now, this provides basic validation with optional bundle verification
type IdentityDocumentValidator struct {
	bundleProvider ports.TrustBundleProvider
}

// NewIdentityDocumentValidator creates a new identity certificate validator
// bundleProvider is optional - if nil, bundle verification is skipped
func NewIdentityDocumentValidator(bundleProvider ports.TrustBundleProvider) *IdentityDocumentValidator {
	return &IdentityDocumentValidator{
		bundleProvider: bundleProvider,
	}
}

// ValidateIdentityDocument checks if an identity certificate is valid for the given identity credential
// This is the anti-corruption layer between domain and SDK validation logic
func (v *IdentityDocumentValidator) ValidateIdentityDocument(ctx context.Context, cert *domain.IdentityDocument, expectedID *domain.IdentityCredential) error {
	if cert == nil {
		return fmt.Errorf("%w: identity document cannot be nil", domain.ErrIdentityDocumentInvalid)
	}

	if expectedID == nil {
		return fmt.Errorf("%w: expected identity credential cannot be nil", domain.ErrInvalidIdentityCredential)
	}

	// Check time validity (using domain method that wraps x509)
	if !cert.IsValid() {
		return fmt.Errorf("%w: certificate expired or not yet valid", domain.ErrIdentityDocumentExpired)
	}

	// Check identity credential matches expected
	if !cert.IdentityCredential().Equals(expectedID) {
		return fmt.Errorf("%w: expected %s, got %s",
			domain.ErrIdentityDocumentMismatch, expectedID.String(), cert.IdentityCredential().String())
	}

	// Bundle verification (optional - if provider is available)
	// TODO: Implement full chain verification with go-spiffe SDK
	if v.bundleProvider != nil {
		bundle, err := v.bundleProvider.GetBundleForIdentity(ctx, cert.IdentityCredential())
		if err != nil {
			return fmt.Errorf("%w: failed to get trust bundle: %v", domain.ErrCertificateChainInvalid, err)
		}

		// In walking skeleton: we have the bundle but skip full chain verification
		// In production with go-spiffe/v2 SDK, this would be:
		//
		// import (
		//     "github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
		//     "github.com/spiffe/go-spiffe/v2/svid/x509svid"
		// )
		//
		// // Build full certificate chain (leaf + intermediates)
		// fullChain := append([]*x509.Certificate{cert.Certificate()}, cert.Chain()...)
		//
		// // Parse PEM bundle as x509bundle.Source for verification
		// // Note: Port returns []byte to stay SDK-agnostic; real adapter could return parsed bundle
		// bundleSource, err := x509bundle.Parse(cert.IdentityCredential().TrustDomain(), bundle)
		// if err != nil {
		//     return fmt.Errorf("%w: failed to parse bundle: %v", domain.ErrCertificateChainInvalid, err)
		// }
		//
		// // Verify certificate chain against trust bundle
		// // For stricter validation, add options like:
		// // opts := []x509svid.VerifyOption{x509svid.WithSVIDConstraint(x509svid.RequireLeaf())}
		// // spiffeID, chains, err := x509svid.Verify(fullChain, bundleSource, opts...)
		// spiffeID, chains, err := x509svid.Verify(fullChain, bundleSource)
		// if err != nil {
		//     return fmt.Errorf("%w: chain verification failed: %v", domain.ErrCertificateChainInvalid, err)
		// }
		//
		// // Validate extracted SPIFFE ID matches expected identity credential
		// if spiffeID.String() != cert.IdentityCredential().String() {
		//     return fmt.Errorf("%w: SPIFFE ID mismatch after verification", domain.ErrIdentityDocumentMismatch)
		// }
		// _ = chains // Verified chains available for further validation if needed

		_ = bundle // Bundle retrieved successfully, verification would happen here with SDK
	}

	return nil
}

// Ensure IdentityDocumentValidator implements the port
var _ ports.IdentityDocumentValidator = (*IdentityDocumentValidator)(nil)
