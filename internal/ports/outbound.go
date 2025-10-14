package ports

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// ConfigLoader loads application configuration
type ConfigLoader interface {
	Load(ctx context.Context) (*Config, error)
}

// NOTE: Dev-only interfaces have been moved to outbound_dev.go
// These interfaces are only available in development builds (//go:build dev tag):
// - IdentityMapperRegistry: Registry for selector-based identity mapping
// - WorkloadAttestor: Platform-specific workload attestation
// - IdentityServer: In-memory identity server for dev/testing
// - IdentityDocumentCreator: Identity document creation (dev only)
// - IdentityDocumentProvider: Composite creator+validator (dev only)
//
// Production builds use external SPIRE infrastructure and only need validation interfaces.

// Agent represents the identity agent functionality for production deployments.
//
// Design Note: In SPIRE deployments, the agent is a long-lived client that:
// - Maintains connection to SPIRE Agent via Unix socket or Workload API
// - Watches for identity updates and rotations
// - Provides identity documents to workloads
//
// The agent must be closed to release resources (socket connections, watchers).
//
// Error Contract (domain sentinels only):
// - GetIdentity returns domain.ErrAgentUnavailable if agent not initialized
// - FetchIdentityDocument returns domain.ErrWorkloadAttestationFailed if attestation fails
// - FetchIdentityDocument returns domain.ErrNoMatchingMapper if no registration matches
// - FetchIdentityDocument returns domain.ErrServerUnavailable if cannot reach server
// - Close returns error if cleanup fails, but is idempotent
type Agent interface {
	// GetIdentity returns the agent's own identity (credential + document + name).
	// Lazily fetches SVID on first call, then caches and proactively refreshes
	// when expiring soon (default: <= 20% lifetime remaining).
	//
	// Returns a shallow copy of the cached identity to discourage mutation.
	// The IdentityDocument within is immutable and safe for concurrent reads.
	GetIdentity(ctx context.Context) (*Identity, error)

	// FetchIdentityDocument fetches an identity document for a workload.
	// Flow: Attest → Match selectors in registry → Issue SVID → Return
	//
	// IMPORTANT: In production SPIRE mode, can only fetch SVID for the calling
	// process (authenticated via Unix socket). The workload parameter is ignored.
	FetchIdentityDocument(ctx context.Context, workload ProcessIdentity) (*domain.IdentityDocument, error)

	// Close releases resources held by the agent (sockets, watchers, sources).
	// This method is idempotent and safe to call multiple times.
	// Should be called via defer after agent creation.
	Close() error
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

// TrustBundleProvider provides trust bundles for X.509 certificate chain validation.
// Trust bundles contain root CA certificates used to verify identity document chains.
//
// Design Note: Returns PEM-encoded bytes to avoid leaking crypto/x509 types into ports.
// Adapters handle parsing PEM → x509.Certificate as needed.
//
// In real SPIRE with go-spiffe SDK:
// - Bundle contains root CAs for trust domain(s)
// - Used by x509svid.Verify(cert, chain, bundle) for chain-of-trust validation
// - Bundles can be federated (multiple trust domains)
//
// Error Contract (domain sentinels only):
// - Returns domain.ErrTrustBundleNotFound if trust domain has no bundle
// - Returns domain.ErrInvalidTrustDomain if trust domain is nil
type TrustBundleProvider interface {
	// GetBundle returns the trust bundle as PEM-encoded bytes for a trust domain
	// Returns concatenated PEM blocks (one or more root CA certificates)
	// Format: "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n"
	// Adapters parse PEM → x509.Certificate for validation operations
	GetBundle(ctx context.Context, trustDomain *domain.TrustDomain) ([]byte, error)

	// GetBundleForIdentity returns the trust bundle as PEM bytes for an identity's trust domain
	// Convenience method that extracts trust domain from identity credential
	// Returns same PEM format as GetBundle
	GetBundleForIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) ([]byte, error)
}

// IdentityDocumentValidator validates identity documents.
// This port abstracts SDK-specific identity document validation (e.g., go-spiffe's x509svid verification).
//
// Design Note: Production workloads only validate received documents.
// Creation is handled by SPIRE Server (external infrastructure).
// For dev/testing, see IdentityDocumentCreator in outbound_dev.go.
//
// The go-spiffe SDK provides mature validation:
// - x509svid.Verify(cert, chain, bundle) for chain-of-trust validation
//
// Error Contract (domain sentinels only):
// - ValidateIdentityDocument returns domain.ErrIdentityDocumentExpired for expired documents
// - ValidateIdentityDocument returns domain.ErrIdentityDocumentMismatch for identity mismatch
// - ValidateIdentityDocument returns domain.ErrCertificateChainInvalid for chain validation failure
type IdentityDocumentValidator interface {
	// ValidateIdentityDocument performs full identity document validation (time, chain-of-trust, signature)
	// Checks identity credential match, expiration, and optionally bundle verification
	// Returns domain sentinel errors (ErrIdentityDocumentExpired, ErrIdentityDocumentInvalid, ErrIdentityDocumentMismatch)
	ValidateIdentityDocument(ctx context.Context, doc *domain.IdentityDocument, expectedID *domain.IdentityCredential) error
}

// BaseAdapterFactory provides minimal adapter creation methods shared by all implementations.
// This interface follows the Interface Segregation Principle by only including
// methods that all implementations (dev and prod) actually use.
//
// Design Note: Returns IdentityDocumentValidator (not full Provider) because:
// - Production implementations can only validate (SPIRE Server handles creation)
// - Development implementations can create, but base interface serves both modes
// - Follows Liskov Substitution Principle - all implementations can validate
type BaseAdapterFactory interface {
	CreateTrustDomainParser() TrustDomainParser
	CreateIdentityCredentialParser() IdentityCredentialParser
	CreateIdentityDocumentValidator() IdentityDocumentValidator
}

// AgentFactory creates SPIRE agents that delegate to external SPIRE infrastructure.
//
// Design Note: SPIRE workloads are clients only (via Workload API).
// SPIRE Server runs as external infrastructure, not embedded in workload processes.
type AgentFactory interface {
	BaseAdapterFactory
	// CreateAgent creates an agent that delegates to external SPIRE.
	// Only requires essential parameters - SPIRE handles registry, attestation, and issuance.
	CreateAgent(ctx context.Context, spiffeID string, parser IdentityCredentialParser) (Agent, error)
}

// AdapterFactory is the primary interface for SPIRE deployments.
// Workloads are clients that fetch their own identity via Workload API.
//
// Design Note: This interface intentionally does NOT include server creation.
// In SPIRE deployments:
//   - SPIRE Server runs as external infrastructure (separate process)
//   - Workloads are clients that communicate via Workload API (unix socket)
//   - Workloads cannot issue arbitrary identities (only fetch their own SVID)
//
// Server functionality is only needed for in-memory/development implementations
// where the "server" runs locally within the process for testing.
type AdapterFactory interface {
	BaseAdapterFactory
	AgentFactory
}

// NOTE: Development mode uses concrete InMemoryAdapterFactory (no interfaces).
// Dev-only code should be simple - interfaces only exist when substitution is needed.
// SPIRE implementations implement AdapterFactory.
