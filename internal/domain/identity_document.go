package domain

import (
	"crypto/x509"
	"time"
)

// IdentityDocument represents an X.509-based verifiable identity document (SVID) for a workload.
// This is a minimal domain type that holds document data without crypto/parsing logic.
// Creation and validation delegated to IdentityDocumentProvider port (implemented in adapters).
//
// Design Note: We avoid duplicating go-spiffe SDK's svid/x509svid package logic
// (ParseX509SVID, validation, chain-of-trust) by moving that to an adapter.
// The domain only models the concept of an identity document with expiration.
//
// The certificate/key/chain are stored for use by adapters (e.g., TLS handshakes,
// message signing). Domain doesn't validate these directly.
//
// Note: This implementation is X.509-only. JWT SVIDs are not supported.
type IdentityDocument struct {
	identityCredential *IdentityCredential
	cert               *x509.Certificate
	privateKey         interface{} // crypto.Signer or crypto.PrivateKey
	chain              []*x509.Certificate
	expiresAt          time.Time
}

// NewIdentityDocumentFromComponents creates an X.509 identity document from already-validated components.
// This is used by the IdentityDocumentProvider adapter after certificate/key validation.
// The cert, privateKey, and chain must be provided; expiresAt is extracted from cert.
func NewIdentityDocumentFromComponents(
	identityCredential *IdentityCredential,
	cert *x509.Certificate,
	privateKey interface{},
	chain []*x509.Certificate,
	expiresAt time.Time,
) *IdentityDocument {
	return &IdentityDocument{
		identityCredential: identityCredential,
		cert:               cert,
		privateKey:         privateKey,
		chain:              chain,
		expiresAt:          expiresAt,
	}
}

// IdentityCredential returns the identity credential
func (id *IdentityDocument) IdentityCredential() *IdentityCredential {
	return id.identityCredential
}

// Certificate returns the X.509 certificate
func (id *IdentityDocument) Certificate() *x509.Certificate {
	return id.cert
}

// PrivateKey returns the private key
func (id *IdentityDocument) PrivateKey() interface{} {
	return id.privateKey
}

// Chain returns the certificate chain
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
