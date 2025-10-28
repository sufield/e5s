package spire

import (
	"crypto/x509"
	"fmt"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"

	"github.com/sufield/e5s/internal/domain"
)

// TranslateX509SVIDToIdentityDocument converts a go-spiffe X509SVID to a domain IdentityDocument.
//
// Note: The domain IdentityDocument no longer stores private keys. Private keys are managed
// separately by adapters (typically in the X509SVID or dto.Identity). This function validates
// the SVID's private key but does not include it in the returned document.
//
// Security hardening (production-grade):
//   - Validates SVID ID is non-zero (defensive)
//   - Validates leaf certificate and all chain entries are non-nil
//   - Requires private key to be a crypto.Signer (usable for mTLS)
//   - Verifies private key matches leaf certificate's public key using x509.Equal
//   - Performs defensive copy of certificate chain (pointer slice only; certs are immutable)
//
// Error contract:
//   - Returns domain.ErrIdentityDocumentInvalid (wrapped with %w) for all validation failures
//   - Returns domain.ErrInvalidIdentityCredential (wrapped) for SPIFFE ID translation errors
//
// Concurrency: Safe for concurrent use (stateless, pure function).
func TranslateX509SVIDToIdentityDocument(svid *x509svid.SVID) (*domain.IdentityDocument, error) {
	if svid == nil {
		return nil, fmt.Errorf("%w: nil SVID", domain.ErrIdentityDocumentInvalid)
	}

	// Defensive: validate SVID ID is non-zero (SDK shouldn't hand us one, but be safe)
	if svid.ID.IsZero() {
		return nil, fmt.Errorf("%w: zero SPIFFE ID", domain.ErrIdentityDocumentInvalid)
	}

	// Validate certificates exist and leaf is non-nil
	if len(svid.Certificates) == 0 || svid.Certificates[0] == nil {
		return nil, fmt.Errorf("%w: missing leaf certificate", domain.ErrIdentityDocumentInvalid)
	}
	leaf := svid.Certificates[0]

	// Ensure no nil entries in certificate chain (defensive against malformed SVIDs)
	for i, cert := range svid.Certificates {
		if cert == nil {
			return nil, fmt.Errorf("%w: nil certificate at position %d", domain.ErrIdentityDocumentInvalid, i)
		}
	}

	// Private key must be present and usable for signing (mTLS requirement)
	// Even though we don't store it in the domain model, we validate it's present and valid
	signer := svid.PrivateKey
	if signer == nil {
		return nil, fmt.Errorf("%w: missing/invalid private key (must be crypto.Signer)", domain.ErrIdentityDocumentInvalid)
	}

	// Verify private key matches leaf certificate's public key
	// Use DER encoding for type-agnostic comparison (handles RSA/ECDSA/Ed25519)
	if !publicKeysEqual(leaf.PublicKey, signer.Public()) {
		return nil, fmt.Errorf("%w: private key does not match leaf certificate", domain.ErrIdentityDocumentInvalid)
	}

	// Convert SPIFFE ID to domain IdentityCredential
	identityCredential, err := TranslateSPIFFEIDToIdentityCredential(svid.ID)
	if err != nil {
		// Preserve sentinel error while adding context
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidIdentityCredential, err)
	}

	// Defensive copy of certificate chain to prevent aliasing bugs
	// Note: Only the slice is copied; cert pointers remain shared (certs are immutable)
	chain := make([]*x509.Certificate, len(svid.Certificates))
	copy(chain, svid.Certificates)

	// Create identity document from SVID components (no private key - managed by adapter)
	return domain.NewIdentityDocumentFromComponents(
		identityCredential,
		leaf,  // Leaf certificate
		chain, // Full chain (leaf + intermediates)
	)
}

// TranslateSPIFFEIDToIdentityCredential converts a go-spiffe ID to a domain IdentityCredential.
//
// Path semantics:
//   - SPIFFE root IDs (e.g., "spiffe://example.org") have no path (id.Path() == "")
//   - Domain model uses "/" to denote root identity
//   - All other paths are preserved exactly as normalized by SDK
//
// Error contract:
//   - Returns domain.ErrInvalidIdentityCredential (wrapped with %w) for zero ID
//
// Concurrency: Safe for concurrent use (stateless, pure function).
func TranslateSPIFFEIDToIdentityCredential(id spiffeid.ID) (*domain.IdentityCredential, error) {
	if id.IsZero() {
		return nil, fmt.Errorf("%w: zero SPIFFE ID", domain.ErrInvalidIdentityCredential)
	}

	// Extract trust domain (SDK-normalized)
	trustDomain := domain.NewTrustDomainFromName(id.TrustDomain().String())

	// Extract path: root IDs have empty path per SPIFFE spec
	// Domain model uses "/" to denote root identity
	path := id.Path()
	if path == "" { // root ID per SPIFFE spec
		path = "/"
	}

	// Create domain IdentityCredential
	return domain.NewIdentityCredentialFromComponents(trustDomain, path), nil
}

// TranslateTrustDomainToSPIFFEID converts a domain TrustDomain to a go-spiffe TrustDomain.
//
// This centralizes trust domain translation to avoid string round-trips scattered
// throughout the codebase and ensures consistent error shaping.
//
// Error contract:
//   - Returns domain.ErrInvalidTrustDomain (wrapped with %w) for nil or malformed input
//   - Chains SDK errors for detailed context
//
// Concurrency: Safe for concurrent use (stateless, pure function).
func TranslateTrustDomainToSPIFFEID(trustDomain *domain.TrustDomain) (spiffeid.TrustDomain, error) {
	if trustDomain == nil {
		return spiffeid.TrustDomain{}, fmt.Errorf("%w: nil trust domain", domain.ErrInvalidTrustDomain)
	}

	// Parse trust domain string into go-spiffe TrustDomain
	// SDK validates DNS name format per SPIFFE spec
	sdkTD, err := spiffeid.TrustDomainFromString(trustDomain.String())
	if err != nil {
		return spiffeid.TrustDomain{}, fmt.Errorf("%w: %w", domain.ErrInvalidTrustDomain, err)
	}

	return sdkTD, nil
}

// TranslateIdentityCredentialToSPIFFEID converts a domain IdentityCredential to a go-spiffe ID.
//
// Design: Uses FromSegments (segment-based construction) instead of FromPath (string-based)
// to avoid accidental double slashes and let the SDK handle all normalization.
//
// Path handling:
//   - Domain path "/" → SPIFFE root ID (zero segments) → "spiffe://example.org"
//   - Non-root paths → split into clean segments using shared helper
//
// Error contract:
//   - Returns domain.ErrInvalidIdentityCredential (wrapped with %w) for nil or malformed input
//   - Chains SDK errors for detailed context
//
// Concurrency: Safe for concurrent use (stateless, pure function).
func TranslateIdentityCredentialToSPIFFEID(identityCredential *domain.IdentityCredential) (spiffeid.ID, error) {
	if identityCredential == nil {
		return spiffeid.ID{}, fmt.Errorf("%w: nil identity credential", domain.ErrInvalidIdentityCredential)
	}

	// Parse trust domain using centralized translator
	sdkTD, err := spiffeid.TrustDomainFromString(identityCredential.TrustDomain().String())
	if err != nil {
		return spiffeid.ID{}, fmt.Errorf("%w: %w", domain.ErrInvalidIdentityCredential, err)
	}

	// Handle root path explicitly (domain uses "/" for root identity)
	if identityCredential.Path() == "/" {
		return spiffeid.FromSegments(sdkTD) // root ID (zero segments)
	}

	// Split path into clean segments using shared helper (deduplication)
	segments := segmentsFromPath(identityCredential.Path())

	// Build SPIFFE ID from segments (type-safe, SDK-normalized)
	return spiffeid.FromSegments(sdkTD, segments...)
}

// publicKeysEqual compares two public keys for equality using DER encoding.
// This is type-agnostic and handles RSA, ECDSA, and Ed25519 keys correctly.
// Returns true if keys are equal, false otherwise (including on encoding errors).
func publicKeysEqual(a, b any) bool {
	derA, errA := x509.MarshalPKIXPublicKey(a)
	derB, errB := x509.MarshalPKIXPublicKey(b)
	if errA != nil || errB != nil {
		return false
	}
	if len(derA) != len(derB) {
		return false
	}
	for i := range derA {
		if derA[i] != derB[i] {
			return false
		}
	}
	return true
}
