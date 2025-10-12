package ports

import (
	"context"
	"crypto/x509"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// ConfigLoader loads application configuration
type ConfigLoader interface {
	Load(ctx context.Context) (*Config, error)
}

// IdentityMapperRegistry provides read-only access to the identity mapper registry seeded at startup
// This is the runtime interface - seeding happens via internal methods during bootstrap
// No mutations allowed after seeding - registry is immutable
//
// Error Contract:
// - FindBySelectors returns domain.ErrNoMatchingMapper if no mapper matches
// - FindBySelectors returns domain.ErrInvalidSelectors if selectors are nil/empty
// - ListAll returns domain.ErrRegistryEmpty if no mappers seeded
type IdentityMapperRegistry interface {
	// FindBySelectors finds an identity mapper matching the given selectors (AND logic)
	// This is the core runtime operation: selectors → identity credential mapping
	// All mapper selectors must be present in discovered selectors for a match
	FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)

	// ListAll returns all seeded identity mappers (for debugging/admin)
	ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}

// WorkloadAttestor verifies workload identity based on platform-specific attributes
//
// Error Contract:
// - Returns domain.ErrWorkloadAttestationFailed if attestation fails
// - Returns domain.ErrInvalidProcessIdentity if workload info is invalid
// - Returns domain.ErrNoAttestationData if no selectors can be generated
type WorkloadAttestor interface {
	// Attest verifies a workload and returns its selectors
	// Selectors format: "type:value" (e.g., "unix:uid:1000", "k8s:namespace:prod")
	Attest(ctx context.Context, workload ProcessIdentity) ([]string, error)
}

// NOTE: NodeAttestor has been moved to node_attestor.go with !production build tag.
// In production deployments using real SPIRE, node attestation is handled by SPIRE Server.

// IdentityServer represents the identity server functionality
//
// Error Contract:
// - IssueIdentity returns domain.ErrIdentityDocumentInvalid if identity credential invalid
// - IssueIdentity returns domain.ErrServerUnavailable if server unavailable
// - IssueIdentity returns domain.ErrCANotInitialized if CA not initialized
// - GetTrustDomain never returns error (returns nil if not initialized)
// - GetCA returns nil if CA not initialized (check before use)
type IdentityServer interface {
	// IssueIdentity issues an identity document for an identity credential
	// Generates X.509 certificate signed by CA with identity credential in URI SAN
	IssueIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.IdentityDocument, error)

	// GetTrustDomain returns the trust domain this server manages
	GetTrustDomain() *domain.TrustDomain

	// GetCA returns the CA certificate (root of trust)
	// Returns nil if CA not initialized - caller must check
	GetCA() *x509.Certificate
}

// Agent represents the identity agent functionality
//
// Error Contract:
// - GetIdentity returns domain.ErrAgentUnavailable if agent not initialized
// - FetchIdentityDocument returns domain.ErrWorkloadAttestationFailed if attestation fails
// - FetchIdentityDocument returns domain.ErrNoMatchingMapper if no registration matches
// - FetchIdentityDocument returns domain.ErrServerUnavailable if cannot reach server
type Agent interface {
	// GetIdentity returns the agent's own identity
	// Agent must bootstrap its identity before serving workloads
	GetIdentity(ctx context.Context) (*Identity, error)

	// FetchIdentityDocument fetches an identity document for a workload
	// Flow: Attest → Match selectors in registry → Issue SVID → Return
	FetchIdentityDocument(ctx context.Context, workload ProcessIdentity) (*Identity, error)
}

// TrustDomainParser parses and validates trust domain strings
// This port abstracts SDK-specific trust domain parsing (e.g., go-spiffe's spiffeid.TrustDomainFromString)
// to avoid duplicating SDK logic in the domain layer.
//
// Design Note: The go-spiffe SDK provides mature trust domain handling:
// - spiffeid.TrustDomainFromString for validation (DNS label checks, no scheme/path)
// - Case-insensitive equality
// - Proper DNS label validation
// By using this port:
// - Real implementation can use SDK for proper validation
// - In-memory implementation can use simple string validation for walking skeleton
// - Domain remains SDK-agnostic (only holds validated data)
type TrustDomainParser interface {
	// FromString parses a trust domain from a string (e.g., "example.org")
	// Validates DNS format, ensures no scheme or path, returns domain.TrustDomain
	FromString(ctx context.Context, name string) (*domain.TrustDomain, error)
}

// IdentityCredentialParser parses and validates identity credential strings
// This port abstracts SDK-specific identity credential parsing (e.g., go-spiffe's spiffeid.FromString)
// to avoid duplicating SDK logic in the domain layer.
//
// Design Note: The go-spiffe SDK provides mature, battle-tested parsing/validation
// via spiffeid.FromString and spiffeid.FromPath. By using this port:
// - Real implementation can use SDK for proper validation (scheme, host format, path normalization)
// - In-memory implementation can use simple string parsing for walking skeleton
// - Domain remains SDK-agnostic (only holds parsed data, doesn't parse)
//
// Error Contract:
// - ParseFromString returns domain.ErrInvalidIdentityCredential if URI is empty or invalid format
// - ParseFromString returns domain.ErrInvalidIdentityCredential if scheme is not "spiffe"
// - ParseFromString returns domain.ErrInvalidIdentityCredential if trust domain is empty
// - ParseFromPath returns domain.ErrInvalidIdentityCredential if trust domain is nil
type IdentityCredentialParser interface {
	// ParseFromString parses an identity credential from a URI string (e.g., "spiffe://example.org/host")
	// Validates scheme, extracts trust domain and path, returns domain.IdentityCredential
	ParseFromString(ctx context.Context, id string) (*domain.IdentityCredential, error)

	// ParseFromPath creates an identity credential from trust domain and path components
	// Ensures path starts with "/", formats as spiffe://<td><path>
	ParseFromPath(ctx context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityCredential, error)
}

// TrustBundleProvider provides trust bundles for X.509 certificate chain validation
// Trust bundles contain root CA certificates used to verify identity document chains
//
// Design Note: In real SPIRE with go-spiffe SDK:
// - Bundle contains root CAs for trust domain(s)
// - Used by x509svid.Verify(cert, chain, bundle) for chain-of-trust validation
// - Bundles can be federated (multiple trust domains)
//
// Error Contract:
// - Returns domain.ErrTrustBundleNotFound if trust domain has no bundle
// - Returns domain.ErrInvalidTrustDomain if trust domain is nil
type TrustBundleProvider interface {
	// GetBundle returns the trust bundle (root CA certificates) for a trust domain
	// Returns map of trust domain string → PEM-encoded CA certificate(s)
	// In production, would use go-spiffe's bundle.Source
	GetBundle(ctx context.Context, trustDomain *domain.TrustDomain) ([]byte, error)

	// GetBundleForIdentity returns the trust bundle for an identity's trust domain
	// Convenience method that extracts trust domain from identity credential
	GetBundleForIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) ([]byte, error)
}

// IdentityDocumentCreator creates identity documents (X.509 SVIDs).
// This port abstracts SDK-specific identity document creation (e.g., go-spiffe's x509svid package).
//
// Design Note: The go-spiffe SDK provides mature document creation:
// - x509svid.ParseX509SVID(certBytes, keyBytes) for DER parsing
// - Proper crypto.Signer handling for private keys
// By using this port:
// - Real implementation can use SDK for proper document handling
// - In-memory implementation can generate simple test documents
//
// Error Contract:
// - CreateX509IdentityDocument returns domain.ErrIdentityDocumentInvalid for invalid inputs
type IdentityDocumentCreator interface {
	// CreateX509IdentityDocument creates an X.509 identity document with certificate and private key
	// Generates certificate signed by CA, extracts expiration, returns domain.IdentityDocument
	// In real implementation: uses SDK's x509svid.Parse or manual x509.CreateCertificate
	CreateX509IdentityDocument(ctx context.Context, identityCredential *domain.IdentityCredential, caCert interface{}, caKey interface{}) (*domain.IdentityDocument, error)
}

// IdentityDocumentValidator validates identity documents.
// This port abstracts SDK-specific identity document validation (e.g., go-spiffe's x509svid verification).
//
// Design Note: The go-spiffe SDK provides mature validation:
// - x509svid.Verify(cert, chain, bundle) for chain-of-trust validation
//
// Error Contract:
// - ValidateIdentityDocument returns domain.ErrIdentityDocumentExpired for expired documents
// - ValidateIdentityDocument returns domain.ErrIdentityDocumentMismatch for identity mismatch
// - ValidateIdentityDocument returns domain.ErrCertificateChainInvalid for chain validation failure
type IdentityDocumentValidator interface {
	// ValidateIdentityDocument performs full identity document validation (time, chain-of-trust, signature)
	// Checks identity credential match, expiration, and optionally bundle verification
	// Returns domain sentinel errors (ErrIdentityDocumentExpired, ErrIdentityDocumentInvalid, ErrIdentityDocumentMismatch)
	ValidateIdentityDocument(ctx context.Context, doc *domain.IdentityDocument, expectedID *domain.IdentityCredential) error
}

// IdentityDocumentProvider combines creation and validation of identity documents.
// This composite interface is provided for adapters that implement both responsibilities.
//
// Naming: "IdentityDocumentProvider" is more inclusive than "CertificateProvider" (encompasses
// both X.509 and JWT formats) while remaining self-explanatory and domain-focused.
type IdentityDocumentProvider interface {
	IdentityDocumentCreator
	IdentityDocumentValidator
}

// BaseAdapterFactory provides minimal adapter creation methods shared by all implementations.
// This interface follows the Interface Segregation Principle by only including
// methods that all implementations (dev and prod) actually use.
type BaseAdapterFactory interface {
	CreateTrustDomainParser() TrustDomainParser
	CreateIdentityCredentialParser() IdentityCredentialParser
	CreateIdentityDocumentProvider() IdentityDocumentProvider
}

// ProductionAgentFactory creates production-oriented agents.
// Production implementations delegate to external SPIRE Agent.
//
// Design Note: Production SPIRE workloads are clients only (via Workload API).
// Real SPIRE Server runs as external infrastructure, not embedded in workload processes.
type ProductionAgentFactory interface {
	BaseAdapterFactory
	// CreateProductionAgent creates an agent that delegates to external SPIRE.
	// Only requires essential parameters - SPIRE handles registry, attestation, and issuance.
	CreateProductionAgent(ctx context.Context, spiffeID string, parser IdentityCredentialParser) (Agent, error)
}

// CoreAdapterFactory is the primary interface for production SPIRE deployments.
// Production workloads are clients that fetch their own identity via Workload API.
//
// Design Note: This interface intentionally does NOT include server creation.
// In production SPIRE deployments:
//   - SPIRE Server runs as external infrastructure (separate process)
//   - Workloads are clients that communicate via Workload API (unix socket)
//   - Workloads cannot issue arbitrary identities (only fetch their own SVID)
//
// Server functionality is only needed for in-memory/development implementations
// where the "server" runs locally within the process for testing.
type CoreAdapterFactory interface {
	BaseAdapterFactory
	ProductionAgentFactory
}

// NOTE: Development-only adapter factory interfaces are defined in outbound_dev.go
// and are excluded from production builds via build tag (//go:build dev).
//
// Available in development builds only:
// - DevelopmentAdapterFactory - extends BaseAdapterFactory with dev-specific methods
// - RegistryConfigurator - provides SeedRegistry, SealRegistry
// - AttestorConfigurator - provides RegisterWorkloadUID
// - AdapterFactory - composite interface for full in-memory mode
//
// Production implementations should implement CoreAdapterFactory.
// Development implementations implement the full AdapterFactory composite.
