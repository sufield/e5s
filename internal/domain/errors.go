package domain

import (
	"errors"
)

// Sentinel errors for domain operations
// These define the contract between domain and ports - all adapters must return these exact errors
// Use with errors.Is() for checking and fmt.Errorf("%w", ...) for wrapping with context

// Registry errors
var (
	// ErrNoMatchingMapper indicates no identity mapper matches the given selectors
	// Used by: IdentityMapperRegistry.FindBySelectors
	ErrNoMatchingMapper = errors.New("no identity mapper found matching selectors")

	// ErrRegistrySealed indicates registry is sealed and cannot accept new entries
	// Used by: InMemoryRegistry.Seed (internal)
	ErrRegistrySealed = errors.New("registry is sealed, cannot seed after bootstrap")

	// ErrRegistryEmpty indicates registry has no entries
	// Used by: IdentityMapperRegistry.ListAll
	ErrRegistryEmpty = errors.New("registry is empty")
)

// Parser and validation errors
var (
	// ErrInvalidIdentityCredential indicates identity credential is nil or malformed
	// Used by: IdentityCredentialParser.ParseFromString, ParseFromPath
	ErrInvalidIdentityCredential = errors.New("invalid identity credential")

	// ErrInvalidTrustDomain indicates trust domain is nil, empty, or malformed
	// Used by: TrustDomainParser.FromString
	ErrInvalidTrustDomain = errors.New("invalid trust domain")

	// ErrInvalidSelectors indicates selectors are nil or empty
	// Used by: IdentityMapperRegistry.FindBySelectors, domain.SelectorSet operations
	ErrInvalidSelectors = errors.New("selectors cannot be nil or empty")

	// ErrSelectorInvalid indicates selector format is invalid
	// Used by: domain.ParseSelectorFromString
	ErrSelectorInvalid = errors.New("invalid selector format")
)

// Identity document errors
var (
	// ErrIdentityDocumentExpired indicates identity document has expired or not yet valid
	// Used by: IdentityDocumentProvider.ValidateIdentityDocument, domain.IdentityDocument.IsValid
	ErrIdentityDocumentExpired = errors.New("identity document is expired or not yet valid")

	// ErrIdentityDocumentInvalid indicates identity document is nil, malformed, or unsigned
	// Used by: IdentityDocumentProvider.CreateX509IdentityDocument, ValidateIdentityDocument
	ErrIdentityDocumentInvalid = errors.New("identity document is invalid")

	// ErrIdentityDocumentMismatch indicates identity document identity credential doesn't match expected
	// Used by: IdentityDocumentProvider.ValidateIdentityDocument
	ErrIdentityDocumentMismatch = errors.New("identity document identity credential mismatch")

	// ErrCertificateChainInvalid indicates certificate chain validation failed
	// Used by: IdentityDocumentProvider.ValidateIdentityDocument (with SDK)
	ErrCertificateChainInvalid = errors.New("certificate chain validation failed")

	// ErrTrustBundleNotFound indicates trust bundle not found for trust domain
	// Used by: TrustBundleProvider.GetBundle
	ErrTrustBundleNotFound = errors.New("trust bundle not found for trust domain")
)

// Attestation errors
var (
	// ErrNodeAttestationFailed indicates node attestation failed
	// Used by: NodeAttestor.AttestNode
	ErrNodeAttestationFailed = errors.New("node attestation failed")

	// ErrWorkloadAttestationFailed indicates workload attestation failed
	// Used by: WorkloadAttestor.Attest
	ErrWorkloadAttestationFailed = errors.New("workload attestation failed")

	// ErrNoAttestationData indicates no attestation data available
	// Used by: WorkloadAttestor.Attest, NodeAttestor.AttestNode
	ErrNoAttestationData = errors.New("no attestation data available")

	// ErrInvalidProcessIdentity indicates process identity is invalid or incomplete
	// Used by: WorkloadAttestor.Attest
	ErrInvalidProcessIdentity = errors.New("invalid process identity")
)

// Server and agent errors
var (
	// ErrServerUnavailable indicates SPIRE server is unavailable
	// Used by: Server.IssueIdentity
	ErrServerUnavailable = errors.New("server is unavailable")

	// ErrAgentUnavailable indicates SPIRE agent is unavailable
	// Used by: Agent.FetchIdentityDocument
	ErrAgentUnavailable = errors.New("agent is unavailable")

	// ErrCANotInitialized indicates CA certificate is not initialized
	// Used by: Server.GetCA, Server.IssueIdentity
	ErrCANotInitialized = errors.New("CA certificate not initialized")
)

// Entity validation errors
var (
	// ErrIdentityMapperInvalid indicates identity mapper validation failed
	// Used by: domain.NewIdentityMapper
	ErrIdentityMapperInvalid = errors.New("identity mapper validation failed")

	// ErrWorkloadInvalid indicates workload validation failed
	// Used by: domain workload validation
	ErrWorkloadInvalid = errors.New("workload validation failed")

	// ErrNodeInvalid indicates node validation failed
	// Used by: domain.Node validation
	ErrNodeInvalid = errors.New("node validation failed")
)
