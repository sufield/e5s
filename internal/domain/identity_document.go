package domain

import (
	"crypto/x509"
	"time"
)

// IdentityDocumentType represents the format of the identity document
type IdentityDocumentType string

const (
	IdentityDocumentTypeX509 IdentityDocumentType = "x509"
	IdentityDocumentTypeJWT  IdentityDocumentType = "jwt"
)

// IdentityDocument represents a verifiable identity document for a workload
// This is a minimal domain type that holds document data without crypto/parsing logic.
// Creation and validation delegated to IdentityDocumentProvider port (implemented in adapters).
//
// Design Note: We avoid duplicating go-spiffe SDK's svid/x509svid package logic
// (ParseX509SVID, validation, chain-of-trust) by moving that to an adapter.
// The domain only models the concept of an identity document with expiration.
//
// For X.509 identity documents, the certificate/key/chain are stored for use by adapters
// (e.g., TLS handshakes, message signing). Domain doesn't validate these directly.
//
// Naming: "IdentityDocument" is more inclusive than "IdentityCertificate" (which implies
// X.509 bias) - it encompasses both X.509 and JWT formats while remaining self-explanatory
// and domain-focused, especially for collaborators unfamiliar with SPIFFE terminology.
type IdentityDocument struct {
	identityNamespace       *IdentityNamespace
	identityDocumentType IdentityDocumentType
	cert                 *x509.Certificate // X.509 only - stored for adapter use
	privateKey           interface{}       // X.509 only - crypto.Signer or crypto.PrivateKey
	chain                []*x509.Certificate
	expiresAt            time.Time
}

// NewIdentityDocumentFromComponents creates an identity document from already-validated components.
// This is used by the IdentityDocumentProvider adapter after document/key validation.
// For X.509: cert, privateKey, chain must be provided; expiresAt extracted from cert
// For JWT: cert/privateKey/chain are nil; expiresAt from JWT claims
func NewIdentityDocumentFromComponents(
	identityNamespace *IdentityNamespace,
	identityDocumentType IdentityDocumentType,
	cert *x509.Certificate,
	privateKey interface{},
	chain []*x509.Certificate,
	expiresAt time.Time,
) *IdentityDocument {
	return &IdentityDocument{
		identityNamespace:       identityNamespace,
		identityDocumentType: identityDocumentType,
		cert:                 cert,
		privateKey:           privateKey,
		chain:                chain,
		expiresAt:            expiresAt,
	}
}

// IdentityNamespace returns the identity namespace
func (id *IdentityDocument) IdentityNamespace() *IdentityNamespace {
	return id.identityNamespace
}

// Type returns the identity document type (X.509 or JWT)
func (id *IdentityDocument) Type() IdentityDocumentType {
	return id.identityDocumentType
}

// Certificate returns the X.509 certificate (nil for JWT identity documents)
func (id *IdentityDocument) Certificate() *x509.Certificate {
	return id.cert
}

// PrivateKey returns the private key (nil for JWT identity documents)
func (id *IdentityDocument) PrivateKey() interface{} {
	return id.privateKey
}

// Chain returns the certificate chain (nil for JWT identity documents)
func (id *IdentityDocument) Chain() []*x509.Certificate {
	return id.chain
}

// ExpiresAt returns the expiration time
func (id *IdentityDocument) ExpiresAt() time.Time {
	return id.expiresAt
}

// IsExpired checks if the identity document has expired based on ExpiresAt
func (id *IdentityDocument) IsExpired() bool {
	return time.Now().After(id.expiresAt)
}

// IsValid checks if the identity document is currently valid (simple time check)
// For full validation (chain-of-trust, signature verification), use IdentityDocumentValidator port
func (id *IdentityDocument) IsValid() bool {
	return !id.IsExpired()
}
