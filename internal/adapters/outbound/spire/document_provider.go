package spire

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

// SDKDocumentProvider implements IdentityDocumentProvider using go-spiffe SDK.
// This provides production-grade certificate validation with full chain-of-trust verification.
//
// Design Note: In production SPIRE deployments:
//   - Certificate CREATION happens on SPIRE Server (this provider returns error if called)
//   - Certificate VALIDATION uses SDK's x509svid.Verify for:
//     * Chain-of-trust verification against trust bundles
//     * Signature validation
//     * Expiration checks
//     * SPIFFE ID extraction and validation
//
// This replaces the lightweight inmemory validator with spec-compliant SDK validation.
type SDKDocumentProvider struct {
	bundleSource x509bundle.Source
}

// NewSDKDocumentProvider creates a new SDK-based document provider.
//
// Parameters:
//   - bundleSource: Source for X.509 trust bundles (typically from SPIRE Workload API)
//
// The bundle source is used to fetch root CA certificates for chain verification.
// In production, this is typically obtained from SPIREClient's bundle watcher.
func NewSDKDocumentProvider(bundleSource x509bundle.Source) ports.IdentityDocumentProvider {
	return &SDKDocumentProvider{
		bundleSource: bundleSource,
	}
}

// CreateX509IdentityDocument is not supported in production.
// Certificate creation is delegated to SPIRE Server.
//
// In production SPIRE deployments, certificates are issued by SPIRE Server
// and fetched via Workload API. This method exists only for interface compliance.
//
// Returns domain.ErrIdentityDocumentInvalid indicating the operation is not supported.
func (p *SDKDocumentProvider) CreateX509IdentityDocument(
	ctx context.Context,
	identityCredential *domain.IdentityCredential,
	caCert interface{},
	caKey interface{},
) (*domain.IdentityDocument, error) {
	return nil, fmt.Errorf("%w: certificate creation is delegated to SPIRE Server in production", domain.ErrIdentityDocumentInvalid)
}

// ValidateIdentityDocument performs full X.509 SVID validation using go-spiffe SDK.
//
// Validation steps:
//  1. Basic null/expiration checks (fast fail)
//  2. Identity credential matching
//  3. SDK chain-of-trust verification using x509svid.Verify:
//     - Validates certificate chain against trust bundle
//     - Verifies signatures
//     - Checks SPIFFE ID in certificate URI SAN
//     - Validates expiration at x509 level
//
// Parameters:
//   - ctx: Context for bundle fetching (timeout, cancellation)
//   - doc: Identity document to validate
//   - expectedID: Expected identity credential (must match certificate's SPIFFE ID)
//
// Returns:
//   - nil if validation succeeds
//   - domain.ErrIdentityDocumentInvalid for nil/malformed inputs
//   - domain.ErrIdentityDocumentExpired if certificate is expired
//   - domain.ErrIdentityDocumentMismatch if identity doesn't match expected
//   - domain.ErrCertificateChainInvalid if chain verification fails
//
// Error Contract: Always returns domain sentinel errors for proper handling by callers.
func (p *SDKDocumentProvider) ValidateIdentityDocument(
	ctx context.Context,
	doc *domain.IdentityDocument,
	expectedID *domain.IdentityCredential,
) error {
	// Step 1: Basic validation (fast fail)
	if doc == nil {
		return fmt.Errorf("%w: identity document cannot be nil", domain.ErrIdentityDocumentInvalid)
	}

	if expectedID == nil {
		return fmt.Errorf("%w: expected identity credential cannot be nil", domain.ErrInvalidIdentityCredential)
	}

	// Step 2: Quick expiration check using domain method
	if doc.IsExpired() {
		return domain.ErrIdentityDocumentExpired
	}

	// Step 3: Identity credential match
	if !doc.IdentityCredential().Equals(expectedID) {
		return fmt.Errorf("%w: expected %s, got %s",
			domain.ErrIdentityDocumentMismatch,
			expectedID.String(),
			doc.IdentityCredential().String())
	}

	// Step 4: Extract certificate chain for SDK verification
	chain := doc.Chain()
	if len(chain) == 0 {
		return fmt.Errorf("%w: certificate chain is empty", domain.ErrIdentityDocumentInvalid)
	}

	// Step 5: Parse trust domain for bundle lookup
	// Avoid redundant string round-trip by parsing once
	trustDomain, err := spiffeid.TrustDomainFromString(expectedID.TrustDomain().String())
	if err != nil {
		return fmt.Errorf("%w: %w", domain.ErrInvalidTrustDomain, err)
	}

	// Step 6: SDK chain-of-trust verification
	// x509svid.Verify performs:
	// - Full x509 path validation against bundle source
	// - Signature verification against trust domain CAs
	// - SPIFFE ID extraction from URI SAN
	// - Expiration validation
	//
	// Note: SDK's Verify accepts bundleSource directly (not individual bundle)
	// and handles trust domain lookup internally
	verifiedID, verifiedChains, err := x509svid.Verify(chain, p.bundleSource)
	if err != nil {
		return fmt.Errorf("%w: chain verification failed: %v", domain.ErrCertificateChainInvalid, err)
	}

	// Step 7: Verify extracted SPIFFE ID matches expected
	// Compare trust domains first (fast fail)
	if verifiedID.TrustDomain().String() != trustDomain.String() {
		return fmt.Errorf("%w: verified trust domain %s does not match expected %s",
			domain.ErrIdentityDocumentMismatch,
			verifiedID.TrustDomain().String(),
			trustDomain.String())
	}

	// Full SPIFFE ID comparison
	// Note: String comparison is appropriate here as SDK normalizes SPIFFE IDs
	if verifiedID.String() != expectedID.String() {
		return fmt.Errorf("%w: verified SPIFFE ID %s does not match expected %s",
			domain.ErrIdentityDocumentMismatch,
			verifiedID.String(),
			expectedID.String())
	}

	// Step 8: Verification successful
	// verifiedChains contains validated certificate chains (available for future policy checks)
	_ = verifiedChains

	return nil
}

// Compile-time interface verification
var _ ports.IdentityDocumentProvider = (*SDKDocumentProvider)(nil)
