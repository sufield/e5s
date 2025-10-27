// Package domain contains the domain model for the SPIRE wrapper.
//
// This package is the CORE of the hexagonal architecture - it defines business
// entities and value objects with ZERO dependencies on external frameworks,
// SDKs, or infrastructure. It focuses on pure business logic and domain concepts.
//
// Hexagonal Architecture Boundaries:
//   - Domain NEVER imports from: internal/adapters, internal/ports, pkg/, external SDKs
//   - Domain ONLY imports from: standard library, other domain types
//   - Domain exposes: value objects, entities, domain errors
//   - Domain does NOT: perform I/O, call external APIs, depend on frameworks
//
// All parsing, validation, crypto, and I/O operations are delegated to adapter
// ports (defined in internal/ports). Domain models remain simple value objects.
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
