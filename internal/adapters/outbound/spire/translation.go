package spire

import (
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

// TranslateX509SVIDToIdentityDocument converts a go-spiffe X509SVID to a domain IdentityDocument.
//
// Security notes:
// - Validates that leaf certificate and private key are present (required for mTLS)
// - Performs defensive copy of certificate chain to prevent aliasing bugs
// - Private key is embedded in the domain document for mTLS operations
func TranslateX509SVIDToIdentityDocument(svid *x509svid.SVID) (*domain.IdentityDocument, error) {
	if svid == nil {
		return nil, domain.ErrIdentityDocumentInvalid
	}

	// Validate certificates exist and leaf is non-nil
	if len(svid.Certificates) == 0 || svid.Certificates[0] == nil {
		return nil, fmt.Errorf("%w: missing leaf certificate", domain.ErrIdentityDocumentInvalid)
	}

	// Validate private key is present
	// The domain IdentityDocument requires a private key for mTLS operations
	if svid.PrivateKey == nil {
		return nil, fmt.Errorf("%w: missing private key", domain.ErrIdentityDocumentInvalid)
	}

	// Convert SPIFFE ID to domain IdentityCredential
	identityCredential, err := TranslateSPIFFEIDToIdentityCredential(svid.ID)
	if err != nil {
		return nil, fmt.Errorf("translate SPIFFE ID: %w", err)
	}

	// Defensive copy of the cert chain to avoid aliasing
	chain := make([]*x509.Certificate, len(svid.Certificates))
	copy(chain, svid.Certificates)

	// Create identity document from SVID components
	return domain.NewIdentityDocumentFromComponents(
		identityCredential,
		chain[0],        // Leaf certificate
		svid.PrivateKey, // Private key for mTLS
		chain,           // Full chain including leaf
		chain[0].NotAfter,
	), nil
}

// TranslateSPIFFEIDToIdentityCredential converts a go-spiffe ID to a domain IdentityCredential.
//
// Path semantics:
// - SPIFFE root IDs (e.g., "spiffe://example.org") have no path (id.Path() == "")
// - Domain model uses "/" to denote root identity
// - All other paths are preserved as-is (e.g., "/workload/server")
func TranslateSPIFFEIDToIdentityCredential(id spiffeid.ID) (*domain.IdentityCredential, error) {
	if id.IsZero() {
		return nil, domain.ErrInvalidIdentityCredential
	}

	// Extract trust domain
	trustDomain := domain.NewTrustDomainFromName(id.TrustDomain().String())

	// Extract path
	// Note: In SPIFFE, a root ID has no path. We use "/" to denote root in our domain model.
	path := id.Path()
	if path == "" {
		path = "/"
	}

	// Create domain IdentityCredential
	return domain.NewIdentityCredentialFromComponents(trustDomain, path), nil
}

// TranslateTrustDomainToSPIFFEID converts a domain TrustDomain to a go-spiffe TrustDomain
func TranslateTrustDomainToSPIFFEID(trustDomain *domain.TrustDomain) (spiffeid.TrustDomain, error) {
	if trustDomain == nil {
		return spiffeid.TrustDomain{}, fmt.Errorf("%w: nil trust domain", domain.ErrInvalidTrustDomain)
	}

	// Parse trust domain string into go-spiffe TrustDomain
	td, err := spiffeid.TrustDomainFromString(trustDomain.String())
	if err != nil {
		return spiffeid.TrustDomain{}, fmt.Errorf("%w: %w", domain.ErrInvalidTrustDomain, err)
	}

	return td, nil
}

// TranslateIdentityCredentialToSPIFFEID converts a domain IdentityCredential to a go-spiffe ID.
//
// Uses SDK constructors (FromSegments/FromPath) to ensure proper normalization and avoid
// redundant string parsing round-trips.
//
// Path handling:
// - Domain path "/" → SPIFFE root ID (e.g., "spiffe://example.org")
// - Relative paths (no leading "/") are automatically prepended with "/"
// - Absolute paths (with leading "/") are used as-is
//
// Examples:
//   - {TrustDomain: "example.org", Path: "/"}              → "spiffe://example.org"
//   - {TrustDomain: "example.org", Path: "/workload"}      → "spiffe://example.org/workload"
//   - {TrustDomain: "example.org", Path: "workload"}       → "spiffe://example.org/workload"
//   - {TrustDomain: "example.org", Path: "/ns/prod/api"}   → "spiffe://example.org/ns/prod/api"
func TranslateIdentityCredentialToSPIFFEID(identityCredential *domain.IdentityCredential) (spiffeid.ID, error) {
	if identityCredential == nil {
		return spiffeid.ID{}, domain.ErrInvalidIdentityCredential
	}

	// Parse trust domain
	td, err := spiffeid.TrustDomainFromString(identityCredential.TrustDomain().String())
	if err != nil {
		return spiffeid.ID{}, fmt.Errorf("%w: %w", domain.ErrInvalidIdentityCredential, err)
	}

	// Handle path semantics: our domain uses "/" to denote root
	path := identityCredential.Path()
	if path == "/" {
		return spiffeid.FromSegments(td) // root ID (no path)
	}

	// Ensure path has leading slash (handles relative paths gracefully)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return spiffeid.FromPath(td, path)
}
