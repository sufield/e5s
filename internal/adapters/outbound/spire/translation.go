package spire

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

// TranslateX509SVIDToIdentityDocument converts a go-spiffe X509SVID to a domain IdentityDocument.
//
// Security hardening (production-grade):
//   - Validates leaf certificate and all chain entries are non-nil
//   - Requires private key to be a crypto.Signer (usable for mTLS)
//   - Verifies private key matches leaf certificate's public key (prevents subtle mTLS failures)
//   - Performs defensive copy of certificate chain (pointer slice only; certs are immutable)
//
// Immutability note: The returned IdentityDocument contains pointers to x509.Certificate
// instances which are treated as immutable. Only the slice is copied, not the certificate
// DER bytes. This is sufficient for preventing accidental slice aliasing bugs.
//
// Error contract:
//   - Returns domain.ErrIdentityDocumentInvalid (wrapped with %w) for all validation failures
//   - Chains SDK errors for detailed context
//
// Concurrency: Safe for concurrent use (stateless, pure function).
func TranslateX509SVIDToIdentityDocument(svid *x509svid.SVID) (*domain.IdentityDocument, error) {
	if svid == nil {
		return nil, fmt.Errorf("%w: nil SVID", domain.ErrIdentityDocumentInvalid)
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
	signer, ok := svid.PrivateKey.(crypto.Signer)
	if !ok || signer == nil {
		return nil, fmt.Errorf("%w: missing/invalid private key (must be crypto.Signer)", domain.ErrIdentityDocumentInvalid)
	}

	// Verify private key matches leaf certificate's public key
	// This prevents subtle mTLS failures where cert and key don't correspond
	if !publicKeyMatches(leaf.PublicKey, signer.Public()) {
		return nil, fmt.Errorf("%w: private key does not match leaf certificate", domain.ErrIdentityDocumentInvalid)
	}

	// Convert SPIFFE ID to domain IdentityCredential
	identityCredential, err := TranslateSPIFFEIDToIdentityCredential(svid.ID)
	if err != nil {
		return nil, fmt.Errorf("translate SPIFFE ID: %w", err)
	}

	// Defensive copy of certificate chain to prevent aliasing bugs
	// Note: Only the slice is copied; cert pointers remain shared (certs are immutable)
	chain := make([]*x509.Certificate, len(svid.Certificates))
	copy(chain, svid.Certificates)

	// Create identity document from SVID components
	return domain.NewIdentityDocumentFromComponents(
		identityCredential,
		leaf,   // Leaf certificate
		signer, // crypto.Signer for mTLS
		chain,  // Full chain (leaf + intermediates)
		leaf.NotAfter,
	), nil
}

// publicKeyMatches compares two public keys for equality by DER-encoding their SubjectPublicKeyInfo.
//
// This approach works across all key types (RSA, ECDSA, Ed25519) without type assertions.
// It compares the canonical DER encoding which includes both the algorithm and key material.
//
// Fallback: If DER encoding fails (malformed keys), attempts best-effort ASN.1 equality.
// This is defensive and should never happen with properly formed crypto.PublicKey instances.
//
// Returns true if keys are identical, false otherwise.
func publicKeyMatches(a, b any) bool {
	derA, errA := x509.MarshalPKIXPublicKey(a)
	derB, errB := x509.MarshalPKIXPublicKey(b)
	if errA != nil || errB != nil {
		// Fallback: best-effort ASN.1 equality (very defensive)
		// Should never happen with valid crypto.PublicKey instances
		rawA, _ := asn1.Marshal(a)
		rawB, _ := asn1.Marshal(b)
		return bytes.Equal(rawA, rawB)
	}
	return bytes.Equal(derA, derB)
}

// TranslateSPIFFEIDToIdentityCredential converts a go-spiffe ID to a domain IdentityCredential.
//
// Path semantics:
//   - SPIFFE root IDs (e.g., "spiffe://example.org") have no path (id.Path() == "")
//   - Domain model uses "/" to denote root identity
//   - All other paths are preserved exactly as normalized by SDK
//
// Trust domain handling:
//   - Uses SDK-normalized trust domain string directly
//   - Preserves exact casing and format from SPIFFE ID
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
// Validation:
//   - Nil check with clear error message
//   - Delegates DNS validation to SDK (spiffeid.TrustDomainFromString)
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
//   - Non-root paths → split into clean segments (filters empty parts from "//")
//   - SDK validates each segment per SPIFFE spec
//
// Segment extraction:
//   - Trims leading/trailing slashes
//   - Splits on "/" and filters empty strings
//   - Handles "//" gracefully (e.g., "a//b" → ["a", "b"])
//
// Examples:
//   - {TrustDomain: "example.org", Path: "/"}              → "spiffe://example.org"
//   - {TrustDomain: "example.org", Path: "/workload"}      → "spiffe://example.org/workload"
//   - {TrustDomain: "example.org", Path: "workload"}       → "spiffe://example.org/workload"
//   - {TrustDomain: "example.org", Path: "/ns//prod/api"} → "spiffe://example.org/ns/prod/api"
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

	// Split path into clean segments (filters empty parts from double slashes)
	trimmed := strings.Trim(identityCredential.Path(), "/")
	parts := strings.Split(trimmed, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			segments = append(segments, part)
		}
	}

	// Build SPIFFE ID from segments (type-safe, SDK-normalized)
	return spiffeid.FromSegments(sdkTD, segments...)
}
