// Package domain contains the domain model for the SPIRE wrapper.
// It focuses on core business concepts without adapter-specific logic or I/O.
// Parsing, validation, and crypto operations are delegated to adapter ports;
// domain models remain simple value objects.
//
// Files and types
// -----------------------
//   - identity_credential.go
//   - IdentityCredential: value object modeling a URI-formatted identity
//     (e.g., SPIFFE ID: spiffe://trust-domain/path). Construction includes
//     path normalization. Parsing is delegated to IdentityCredentialParser adapters.
//
//   - identity_document.go
//   - IdentityDocument: container for identity credentials with expiration
//     and validity logic. Delegates crypto/parsing to IdentityDocumentProvider
//     and IdentityDocumentValidator adapter ports.
//
//   - trust_domain.go
//   - TrustDomain: value object representing the trust domain namespace
//     for identities (e.g., "example.org" in spiffe://example.org/path).
//     Construction and validation delegated to TrustDomainParser adapters.
//
//   - workload.go
//   - Workload: represents a process requesting an identity. Contains
//     process information (PID, UID, GID, path) used for attestation.
//
//   - errors.go
//   - Domain-specific error types for identity credential, document,
//     and trust domain operations.
//
// Design principles
//   - Domain types avoid depending on SDK specifics (go-spiffe). Adapters
//     implement parsing/verification and translate into these value objects.
//   - All I/O, crypto, and external SDK integration happens in adapters,
//     not in the domain layer.
//   - Domain focuses on business rules and invariants (see docs/architecture/INVARIANTS.md).
package domain
