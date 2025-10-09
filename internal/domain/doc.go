// Package domain contains the domain model for the walking-skeleton.
// It focuses on business concepts without any
// adapter-specific logic or I/O. Parsing, validation, and crypto are delegated
// to adapter ports; domain models remain simple value objects and services.
//
// Files and types
// -----------------------
//   - attestation.go
//   - NodeAttestationResult, WorkloadAttestationResult: result objects used
//     by attestation flows.
//   - AttestationService: domain service for matching selectors to
//     identity mappers (MatchWorkloadToMapper).
//   - identity_document.go
//   - IdentityDocument: a minimal container for identity documents (X.509
//     or JWT). Includes expiration logic but delegates crypto/parsing to
//     adapters via the IdentityDocumentProvider/Validator ports.
//   - identity_mapper.go
//   - IdentityMapper: maps selector sets to identity credentials. Provides
//     matching logic via MatchesSelectors.
//   - identity_credential.go
//   - IdentityCredential: value object modeling a URI-formatted identity
//     (e.g., SPIFFE ID). Construction and validation are handled by
//     IdentityCredentialParser adapters.
//   - node.go
//   - Node: represents a host machine/environment and holds selectors and
//     attestation status.
//   - selector.go
//   - Selector, SelectorSet: primitives for expressing and matching
//     selectors (e.g., `unix:uid:1000`, `k8s:ns:default`). Includes
//     parsing helpers and set operations.
//   - trust_domain.go
//   - TrustDomain: value object representing the trust domain namespace
//     for identities.
//   - workload.go
//   - Workload: simple struct representing a process requesting an identity
//     (pid/uid/gid/path).
//
// notes
//   - Domain types avoid depending on SDK specifics (go-spiffe) — adapters
//     implement parsing and verification and translate into these value
//     objects. This keeps the domain small and focused on business rules.
//   - Keep all I/O and crypto out of this package; use ports/adapters for that
//     functionality.
package domain
