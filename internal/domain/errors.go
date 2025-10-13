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
// Error Wrapping Pattern:
//
//	// Good: Single wrap with context
//	return fmt.Errorf("%w: chain verification failed: %v", ErrCertificateChainInvalid, sdkErr)
//
//	// Good: Preserve underlying error with %w
//	return fmt.Errorf("%w: %w", ErrInvalidTrustDomain, sdkErr)
//
//	// Bad: Double-wrapping in one call
//	return fmt.Errorf("%w: %w: chain failed", ErrCertificateChainInvalid, ErrIdentityDocumentInvalid)
//
// Stability: These sentinels define the contract between domain and ports.
// All adapters must return these exact errors for consistent error handling.

// Registry errors
var (
	// ErrNoMatchingMapper indicates no identity mapper matches the given selectors.
	// Used by: IdentityMapperRegistry.FindBySelectors, AttestationService.MatchWorkloadToMapper
	ErrNoMatchingMapper = errors.New("no matching mapper")

	// ErrRegistrySealed indicates registry is sealed and cannot accept new entries.
	// Used by: InMemoryRegistry.Seed (internal)
	ErrRegistrySealed = errors.New("registry sealed")

	// ErrRegistryEmpty indicates registry has no entries.
	// Used by: IdentityMapperRegistry.ListAll
	ErrRegistryEmpty = errors.New("registry empty")
)

// Parser and validation errors
var (
	// ErrInvalidIdentityCredential indicates identity credential is nil or malformed.
	// Used by: IdentityCredentialParser.ParseFromString, ParseFromPath
	ErrInvalidIdentityCredential = errors.New("invalid identity credential")

	// ErrInvalidTrustDomain indicates trust domain is nil, empty, or malformed.
	// Used by: TrustDomainParser.FromString
	ErrInvalidTrustDomain = errors.New("invalid trust domain")

	// ErrInvalidSelectors indicates selectors are nil or empty.
	// Used by: IdentityMapperRegistry.FindBySelectors, AttestationService.MatchWorkloadToMapper
	ErrInvalidSelectors = errors.New("invalid selectors")

	// ErrSelectorInvalid indicates selector format is invalid.
	// Used by: domain.ParseSelectorFromString, domain.NewSelector
	//
	// This consolidates fine-grained selector errors (empty key/value, invalid format).
	// Call sites add specific context via wrapping:
	//   fmt.Errorf("%w: key is empty", ErrSelectorInvalid)
	//   fmt.Errorf("%w: value is empty", ErrSelectorInvalid)
	//   fmt.Errorf("%w: missing colon separator", ErrSelectorInvalid)
	ErrSelectorInvalid = errors.New("invalid selector")
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

// Attestation errors
var (
	// ErrWorkloadAttestationFailed indicates workload attestation failed.
	// Used by: WorkloadAttestor.Attest
	ErrWorkloadAttestationFailed = errors.New("workload attestation failed")

	// ErrNoAttestationData indicates no attestation data available.
	// Used by: WorkloadAttestor.Attest, IdentityProvider.FetchX509SVID
	ErrNoAttestationData = errors.New("no attestation data")

	// ErrInvalidProcessIdentity indicates process identity is invalid or incomplete.
	// Used by: WorkloadAttestor.Attest
	ErrInvalidProcessIdentity = errors.New("invalid process identity")
)

// Server and agent errors
var (
	// ErrServerUnavailable indicates SPIRE server is unavailable.
	// Used by: Server.IssueIdentity
	ErrServerUnavailable = errors.New("server unavailable")

	// ErrAgentUnavailable indicates SPIRE agent is unavailable.
	// Used by: Agent.FetchIdentityDocument, SPIREClient.FetchX509SVID
	ErrAgentUnavailable = errors.New("agent unavailable")

	// ErrCANotInitialized indicates CA certificate is not initialized.
	// Used by: Server.GetCA, Server.IssueIdentity
	ErrCANotInitialized = errors.New("CA not initialized")
)

// Entity validation errors
var (
	// ErrIdentityMapperInvalid indicates identity mapper validation failed.
	// Used by: domain.NewIdentityMapper
	ErrIdentityMapperInvalid = errors.New("identity mapper invalid")

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
	_ error = ErrNoMatchingMapper
	_ error = ErrRegistrySealed
	_ error = ErrRegistryEmpty
	_ error = ErrInvalidIdentityCredential
	_ error = ErrInvalidTrustDomain
	_ error = ErrInvalidSelectors
	_ error = ErrSelectorInvalid
	_ error = ErrIdentityDocumentExpired
	_ error = ErrIdentityDocumentInvalid
	_ error = ErrIdentityDocumentMismatch
	_ error = ErrCertificateChainInvalid
	_ error = ErrTrustBundleNotFound
	_ error = ErrWorkloadAttestationFailed
	_ error = ErrNoAttestationData
	_ error = ErrInvalidProcessIdentity
	_ error = ErrServerUnavailable
	_ error = ErrAgentUnavailable
	_ error = ErrCANotInitialized
	_ error = ErrIdentityMapperInvalid
	_ error = ErrWorkloadInvalid
	_ error = ErrNotSupported
)
