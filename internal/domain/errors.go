package domain

import (
	"errors"
)

// Sentinel errors for domain operations.
//
// Usage Contract:
//   - Adapters wrap underlying errors with the most specific domain sentinel using %w
//   - Wrap exactly once per layer with the most relevant sentinel
//   - Add human-readable context in the wrap text at call sites
//   - Callers branch with errors.Is(err, <sentinel>) or errors.As
//   - Do not export adapter-specific error types beyond these domain sentinels
//
// Error Wrapping:
//   Wrap with single sentinel and context:
//     return fmt.Errorf("%w: chain verification failed: %v", ErrCertificateChainInvalid, sdkErr)
//   Preserve underlying error chain:
//     return fmt.Errorf("%w: %w", ErrInvalidTrustDomain, sdkErr)
//
// Stability: These sentinels define the contract between domain and ports.
// All adapters must return these exact errors for consistent error handling.

// REMOVED: Registry errors - inmemory registry deleted per simplification.md
// The following errors are no longer needed:
//   - ErrNoMatchingMapper (IdentityMapperRegistry deleted)
//   - ErrRegistrySealed (InMemoryRegistry deleted)
//   - ErrRegistryEmpty (IdentityMapperRegistry deleted)

// Parser and validation errors
var (
	// ErrInvalidIdentityCredential indicates identity credential is nil or malformed.
	// Used by: IdentityCredentialParser.ParseFromString, ParseFromPath
	ErrInvalidIdentityCredential = errors.New("invalid identity credential")

	// ErrInvalidTrustDomain indicates trust domain is nil, empty, or malformed.
	// Used by: TrustDomainParser.FromString
	ErrInvalidTrustDomain = errors.New("invalid trust domain")

	// REMOVED: Selector errors - selector types deleted per simplification.md
	// The following errors are no longer needed:
	//   - ErrInvalidSelectors (selector matching deleted)
	//   - ErrSelectorInvalid (selector types deleted)
)

// Identity document errors
var (
	// ErrIdentityDocumentExpired indicates identity document has expired or not yet valid.
	// Used by: IdentityDocumentProvider.ValidateIdentityDocument, IdentityDocument.IsValid
	ErrIdentityDocumentExpired = errors.New("identity document expired")

	// ErrIdentityDocumentInvalid indicates identity document is nil, malformed, or invalid.
	// Used by: IdentityDocumentProvider.CreateX509IdentityDocument, ValidateIdentityDocument
	ErrIdentityDocumentInvalid = errors.New("identity document invalid")

	// ErrIdentityDocumentMismatch indicates identity document doesn't match expected credential.
	// Used by: IdentityDocumentProvider.ValidateIdentityDocument
	ErrIdentityDocumentMismatch = errors.New("identity document mismatch")

	// ErrCertificateChainInvalid indicates certificate chain validation failed.
	// Used by: IdentityDocumentProvider.ValidateIdentityDocument (SDK verification)
	ErrCertificateChainInvalid = errors.New("certificate chain invalid")

	// ErrTrustBundleNotFound indicates trust bundle not found for trust domain.
	// Used by: TrustBundleProvider.GetBundle
	ErrTrustBundleNotFound = errors.New("trust bundle not found")
)

// REMOVED: Attestation errors - inmemory attestation deleted per simplification.md
// The following errors are no longer needed:
//   - ErrWorkloadAttestationFailed (WorkloadAttestor deleted)
//   - ErrNoAttestationData (inmemory attestation deleted)
//   - ErrInvalidProcessIdentity (inmemory attestation deleted)
//
// Production SPIRE integration handles attestation via Workload API.

// REMOVED: Infrastructure errors moved to internal/ports/errors.go
//
// The following errors have been moved per architecture review (docs/5.md):
//   - ErrServerUnavailable → ports.ErrServerUnavailable
//   - ErrAgentUnavailable → ports.ErrAgentUnavailable
//   - ErrCANotInitialized → ports.ErrCANotInitialized
//
// Rationale: These represent infrastructure/adapter concerns, not domain concepts.
// Domain errors should be semantic/business failures only.
//
// Adapters should use ports.Err* variants. If infrastructure errors need to be
// surfaced to application layer, adapters can wrap them with domain errors when
// they have domain meaning (e.g., wrap ports.ErrAgentUnavailable with
// ErrNoAttestationData if attestation fails due to agent being down).

// Entity validation errors
var (
	// REMOVED: ErrIdentityMapperInvalid - IdentityMapper type deleted per simplification.md

	// ErrWorkloadInvalid indicates workload validation failed.
	// Used by: domain workload validation
	ErrWorkloadInvalid = errors.New("workload invalid")
)

// Operation errors
var (
	// ErrNotSupported indicates an operation is not supported in this mode/context.
	// Used by: Production adapters that disable certain operations (e.g., certificate creation)
	//
	// This provides explicit branching for unsupported operations instead of overloading
	// ErrIdentityDocumentInvalid. Callers can distinguish between invalid input vs
	// unsupported operation.
	//
	// Example:
	//   Production SVID provider: Certificate creation delegated to SPIRE Server
	//   return fmt.Errorf("%w: certificate creation in production", ErrNotSupported)
	ErrNotSupported = errors.New("operation not supported")
)

// Error class helpers for grouped error checking.
//
// These provide ergonomic helpers for checking related error classes without
// proliferating many single-purpose Is* functions. Use these when you need to
// handle a category of errors uniformly.

// IsIdentityDocumentError checks if the error is any identity document related failure.
//
// Returns true for:
//   - ErrIdentityDocumentInvalid (malformed, nil, missing fields)
//   - ErrIdentityDocumentExpired (expired or not yet valid)
//   - ErrIdentityDocumentMismatch (credential doesn't match expected)
//   - ErrCertificateChainInvalid (chain verification failed)
//
// Use this when you want to handle all identity document failures uniformly,
// such as logging, metrics, or fallback behavior.
//
// Example:
//
//	if IsIdentityDocumentError(err) {
//	    log.Warn("identity document validation failed", "error", err)
//	    return fallbackIdentity()
//	}
func IsIdentityDocumentError(err error) bool {
	return errors.Is(err, ErrIdentityDocumentInvalid) ||
		errors.Is(err, ErrIdentityDocumentExpired) ||
		errors.Is(err, ErrIdentityDocumentMismatch) ||
		errors.Is(err, ErrCertificateChainInvalid)
}

// Compile-time guards to ensure sentinel errors implement error interface.
// This prevents accidental redefinition and makes intent explicit in code review.
var (
	// Parser and validation errors
	_ error = ErrInvalidIdentityCredential
	_ error = ErrInvalidTrustDomain

	// Identity document errors
	_ error = ErrIdentityDocumentExpired
	_ error = ErrIdentityDocumentInvalid
	_ error = ErrIdentityDocumentMismatch
	_ error = ErrCertificateChainInvalid
	_ error = ErrTrustBundleNotFound

	// Entity validation errors
	_ error = ErrWorkloadInvalid

	// Operation errors
	_ error = ErrNotSupported

	// REMOVED: Registry errors (inmemory deleted)
	// REMOVED: Selector errors (selector types deleted)
	// REMOVED: Attestation errors (inmemory attestation deleted)
	// REMOVED: ErrIdentityMapperInvalid (IdentityMapper deleted)
	// REMOVED: Infrastructure errors (moved to ports.Err*)
)
