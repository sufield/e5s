package spire

import (
	"context"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
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
//
// Concurrency: Safe for concurrent use. The bundleSource must also be safe for concurrent use
// (which is true for SPIRE's X509Source).
//
// Clock Skew: Allows 5 minutes of tolerance for certificate validity checks to handle
// production clock drift between systems.
type SDKDocumentProvider struct {
	bundleSource x509bundle.Source
	clock        func() time.Time // For testability; defaults to time.Now
	clockSkew    time.Duration    // Clock skew tolerance; defaults to 5 minutes
}

// NewSDKDocumentProvider creates a new SDK-based document provider.
//
// Parameters:
//   - bundleSource: Source for X.509 trust bundles (typically from SPIRE Workload API)
//     Must be safe for concurrent use (SPIRE's X509Source is).
//
// The bundle source is used to fetch root CA certificates for chain verification.
// In production, this is typically obtained from SPIREClient's bundle watcher.
//
// Defaults: Clock skew tolerance of 5 minutes, time.Now for clock.
func NewSDKDocumentProvider(bundleSource x509bundle.Source) ports.IdentityDocumentValidator {
	return &SDKDocumentProvider{
		bundleSource: bundleSource,
		clock:        time.Now,
		clockSkew:    5 * time.Minute,
	}
}

// ValidateIdentityDocument performs full X.509 SVID validation using go-spiffe SDK.
// Production deployments only validate certificates; creation is delegated to SPIRE Server.
//
// Validation steps:
//  1. Nil checks (document, expected ID)
//  2. Leaf certificate extraction and validation
//  3. Clock skew tolerant time checks (NotBefore/NotAfter)
//  4. Identity credential matching
//  5. Full chain assembly (leaf + intermediates)
//  6. Nil chain validation after filtering
//  7. Bundle source validation (fail fast if misconfigured)
//  8. SDK chain-of-trust verification using x509svid.Verify:
//     - Validates certificate chain against trust bundle
//     - Verifies signatures
//     - Checks SPIFFE ID in certificate URI SAN
//     - Validates expiration at x509 level
//  9. Strict SPIFFE ID comparison (verified vs expected)
//
// Parameters:
//   - ctx: Context for bundle fetching (timeout, cancellation)
//   - doc: Identity document to validate
//   - expectedID: Expected identity credential (must match certificate's SPIFFE ID)
//
// Returns:
//   - nil if validation succeeds
//   - domain.ErrCertificateChainInvalid if bundle source is nil or chain verification fails
//   - domain.ErrIdentityDocumentInvalid for nil/malformed inputs
//   - domain.ErrIdentityDocumentExpired if certificate is expired (with skew tolerance)
//   - domain.ErrIdentityDocumentMismatch if identity doesn't match expected
//
// Error Contract: Always wraps errors with %w for errors.Is/As compatibility.
//
// Clock Skew: Allows 5 minutes of tolerance for NotBefore/NotAfter to handle production
// clock drift. The SDK also validates times; these checks provide clearer error messages.
func (p *SDKDocumentProvider) ValidateIdentityDocument(
	ctx context.Context,
	doc *domain.IdentityDocument,
	expectedID *domain.IdentityCredential,
) error {
	// Step 1: Basic nil checks (fast fail before expensive operations)
	if doc == nil {
		return fmt.Errorf("%w: identity document cannot be nil", domain.ErrIdentityDocumentInvalid)
	}

	if expectedID == nil {
		return fmt.Errorf("%w: expected identity credential cannot be nil", domain.ErrInvalidIdentityCredential)
	}

	// Step 2: Extract leaf certificate (SDK expects leaf first in chain)
	leaf := doc.Certificate()
	if leaf == nil {
		return fmt.Errorf("%w: missing leaf certificate", domain.ErrIdentityDocumentInvalid)
	}

	// Step 3: Time validation with clock skew tolerance
	// These checks provide clearer errors than SDK's generic validation failures
	now := p.clock()
	skew := p.clockSkew

	// Check NotBefore with skew tolerance (allow up to 5min clock drift)
	if now.Before(leaf.NotBefore.Add(-skew)) {
		return fmt.Errorf("%w: certificate not yet valid (NotBefore: %s, now: %s, skew: %s)",
			domain.ErrIdentityDocumentInvalid,
			leaf.NotBefore.Format(time.RFC3339),
			now.Format(time.RFC3339),
			skew)
	}

	// Check NotAfter with skew tolerance
	if now.After(leaf.NotAfter.Add(skew)) {
		return fmt.Errorf("%w: certificate expired (NotAfter: %s, now: %s, skew: %s)",
			domain.ErrIdentityDocumentExpired,
			leaf.NotAfter.Format(time.RFC3339),
			now.Format(time.RFC3339),
			skew)
	}

	// Step 4: Identity credential match (fast fail before expensive crypto)
	if !doc.IdentityCredential().Equals(expectedID) {
		return fmt.Errorf("%w: expected %s, got %s",
			domain.ErrIdentityDocumentMismatch,
			expectedID,
			doc.IdentityCredential())
	}

	// Step 5: Assemble full chain (leaf + intermediates)
	// IMPORTANT: SDK expects [leaf, intermediate1, intermediate2, ...]
	// Our IdentityDocument stores leaf separately from Chain() (intermediates)
	intermediates := doc.Chain()
	fullChain := make([]*x509.Certificate, 1, 1+len(intermediates))
	fullChain[0] = leaf

	// Filter nil entries from intermediates (defensive)
	for _, cert := range intermediates {
		if cert != nil {
			fullChain = append(fullChain, cert)
		}
	}

	// Step 6: Validate we have at least the leaf
	if len(fullChain) == 0 {
		return fmt.Errorf("%w: empty certificate chain after filtering", domain.ErrIdentityDocumentInvalid)
	}

	// Step 7: Validate bundle source before SDK call (fail fast if misconfigured)
	if p.bundleSource == nil {
		return fmt.Errorf("%w: bundle source is nil", domain.ErrCertificateChainInvalid)
	}

	// Step 8: SDK chain-of-trust verification
	// x509svid.Verify performs:
	// - Full x509 path validation against bundle source
	// - Signature verification against trust domain CAs
	// - SPIFFE ID extraction from URI SAN
	// - Expiration validation (redundant with our checks but authoritative)
	//
	// Note: SDK's Verify accepts bundleSource directly (not individual bundle)
	// and handles trust domain lookup internally
	verifiedID, _, err := x509svid.Verify(fullChain, p.bundleSource)
	if err != nil {
		return fmt.Errorf("%w: chain verification failed: %w", domain.ErrCertificateChainInvalid, err)
	}

	// Step 9: Strict SPIFFE ID comparison (verified vs expected)
	// Use String() comparison as SDK normalizes SPIFFE IDs
	if verifiedID.String() != expectedID.String() {
		return fmt.Errorf("%w: verified SPIFFE ID %s does not match expected %s",
			domain.ErrIdentityDocumentMismatch,
			verifiedID,
			expectedID)
	}

	// Validation successful
	return nil
}

// Compile-time interface verification
var _ ports.IdentityDocumentValidator = (*SDKDocumentProvider)(nil)
