package domain

import (
	"errors"
)

// Sentinel errors for common domain failures
// Use with errors.Is() for checking and fmt.Errorf("%w", ...) for wrapping with context

var (
	// ErrNoMatchingMapper indicates no identity mapper matches the given selectors
	ErrNoMatchingMapper = errors.New("no identity mapper found matching selectors")

	// ErrInvalidSelectors indicates selectors are nil or empty
	ErrInvalidSelectors = errors.New("selectors cannot be nil or empty")

	// ErrInvalidIdentityNamespace indicates identity format is nil or malformed
	ErrInvalidIdentityNamespace = errors.New("identity format cannot be nil")

	// ErrInvalidTrustDomain indicates trust domain is nil or empty
	ErrInvalidTrustDomain = errors.New("trust domain cannot be nil or empty")

	// ErrNodeAttestationFailed indicates node attestation failed
	ErrNodeAttestationFailed = errors.New("node attestation failed")

	// ErrWorkloadAttestationFailed indicates workload attestation failed
	ErrWorkloadAttestationFailed = errors.New("workload attestation failed")

	// ErrIdentityDocumentExpired indicates identity document has expired
	ErrIdentityDocumentExpired = errors.New("identity document is expired or not yet valid")

	// ErrIdentityDocumentInvalid indicates identity document is nil or invalid
	ErrIdentityDocumentInvalid = errors.New("identity document is invalid")

	// ErrIdentityDocumentMismatch indicates identity document identity format doesn't match expected ID
	ErrIdentityDocumentMismatch = errors.New("identity document identity format mismatch")
)

// Validation errors for specific entities

var (
	// ErrIdentityMapperInvalid indicates identity mapper validation failed
	ErrIdentityMapperInvalid = errors.New("identity mapper validation failed")

	// ErrSelectorInvalid indicates selector validation failed
	ErrSelectorInvalid = errors.New("selector validation failed")

	// ErrWorkloadInvalid indicates workload validation failed
	ErrWorkloadInvalid = errors.New("workload validation failed")

	// ErrNodeInvalid indicates node validation failed
	ErrNodeInvalid = errors.New("node validation failed")
)
