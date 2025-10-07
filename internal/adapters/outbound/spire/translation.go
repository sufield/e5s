package spire

import (
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

// TranslateX509SVIDToIdentityDocument converts a go-spiffe X509SVID to a domain IdentityDocument
func TranslateX509SVIDToIdentityDocument(svid *x509svid.SVID) (*domain.IdentityDocument, error) {
	if svid == nil {
		return nil, domain.ErrIdentityDocumentInvalid
	}

	// Validate certificates exist
	if len(svid.Certificates) == 0 {
		return nil, domain.ErrIdentityDocumentInvalid
	}

	// Convert SPIFFE ID to domain IdentityNamespace
	identityNamespace, err := TranslateSPIFFEIDToIdentityNamespace(svid.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to translate SPIFFE ID: %w", err)
	}

	// Create identity document from SVID components
	return domain.NewIdentityDocumentFromComponents(
		identityNamespace,
		domain.IdentityDocumentTypeX509,
		svid.Certificates[0], // Leaf certificate
		svid.PrivateKey,
		svid.Certificates, // Full chain including leaf
		svid.Certificates[0].NotAfter,
	), nil
}

// TranslateSPIFFEIDToIdentityNamespace converts a go-spiffe ID to a domain IdentityNamespace
func TranslateSPIFFEIDToIdentityNamespace(id spiffeid.ID) (*domain.IdentityNamespace, error) {
	if id.IsZero() {
		return nil, domain.ErrInvalidIdentityNamespace
	}

	// Extract trust domain
	trustDomain := domain.NewTrustDomainFromName(id.TrustDomain().String())

	// Extract path
	path := id.Path()
	if path == "" {
		path = "/"
	}

	// Create domain IdentityNamespace
	return domain.NewIdentityNamespaceFromComponents(trustDomain, path), nil
}

// TranslateTrustDomainToSPIFFEID converts a domain TrustDomain to a go-spiffe TrustDomain
func TranslateTrustDomainToSPIFFEID(trustDomain *domain.TrustDomain) (spiffeid.TrustDomain, error) {
	if trustDomain == nil {
		return spiffeid.TrustDomain{}, domain.ErrInvalidTrustDomain
	}

	// Parse trust domain string into go-spiffe TrustDomain
	td, err := spiffeid.TrustDomainFromString(trustDomain.String())
	if err != nil {
		return spiffeid.TrustDomain{}, fmt.Errorf("failed to parse trust domain: %w", err)
	}

	return td, nil
}

// TranslateIdentityNamespaceToSPIFFEID converts a domain IdentityNamespace to a go-spiffe ID
func TranslateIdentityNamespaceToSPIFFEID(identityNamespace *domain.IdentityNamespace) (spiffeid.ID, error) {
	if identityNamespace == nil {
		return spiffeid.ID{}, domain.ErrInvalidIdentityNamespace
	}

	// Parse the full SPIFFE ID string
	id, err := spiffeid.FromString(identityNamespace.String())
	if err != nil {
		return spiffeid.ID{}, fmt.Errorf("failed to parse SPIFFE ID: %w", err)
	}

	return id, nil
}
