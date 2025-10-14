//go:build dev

package ports

import (
	"context"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// IdentityMapperRegistry provides read-only access to the identity mapper registry seeded at startup.
// This interface is only available in development builds for in-memory implementations.
//
// In production deployments, SPIRE Server manages registration entries. Workloads only fetch
// their identity via Workload API - no local registry or selector matching is needed.
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

// WorkloadAttestor verifies workload identity based on platform-specific attributes.
// This interface is only available in development builds for in-memory attestation.
//
// In production deployments, SPIRE Agent performs attestation. Workloads connect
// to the agent's Unix socket, and the agent extracts credentials and attests automatically.
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

// IdentityServer represents identity server functionality for in-memory/dev mode only.
// This interface is only available in development builds.
//
// In production deployments, SPIRE Server runs as external infrastructure (separate process).
// Workloads are clients that communicate via Workload API. This interface exists only
// for dev/testing where the "server" runs locally within the process.
//
// Error Contract:
// - IssueIdentity returns domain.ErrIdentityDocumentInvalid if identity credential invalid
// - IssueIdentity returns domain.ErrServerUnavailable if server unavailable
// - IssueIdentity returns domain.ErrCANotInitialized if CA not initialized
// - GetTrustDomain never returns error (returns nil if not initialized)
// - GetCACertPEM returns empty slice if CA not initialized
type IdentityServer interface {
	// IssueIdentity issues an identity document for an identity credential
	// Generates X.509 certificate signed by CA with identity credential in URI SAN
	IssueIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.IdentityDocument, error)

	// GetTrustDomain returns the trust domain this server manages
	GetTrustDomain() *domain.TrustDomain

	// GetCACertPEM returns the CA certificate as PEM bytes (root of trust)
	// Returns empty slice if CA not initialized - caller must check
	GetCACertPEM() []byte
}

// IdentityDocumentCreator creates identity documents (X.509 SVIDs).
// This interface is only available in development builds.
//
// In production deployments, SPIRE Server handles creation. Workloads only
// validate received documents. This interface exists only for dev/testing.
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
// - CreateX509IdentityDocument returns domain.ErrCANotInitialized if CA not available
type IdentityDocumentCreator interface {
	// CreateX509IdentityDocument creates an X.509 identity document with certificate and private key
	// Generates certificate signed by CA, extracts expiration, returns domain.IdentityDocument
	// In real implementation: uses SDK's x509svid.Parse or manual x509.CreateCertificate
	//
	// Note: caCert and caKey are interface{} to avoid leaking crypto/x509 types into ports.
	// Implementations cast to appropriate types (*x509.Certificate, crypto.Signer).
	CreateX509IdentityDocument(ctx context.Context, identityCredential *domain.IdentityCredential, caCert interface{}, caKey interface{}) (*domain.IdentityDocument, error)
}

// IdentityDocumentProvider combines creation and validation of identity documents.
// This composite interface is only available in development builds.
//
// In production, use IdentityDocumentValidator (from outbound.go) which is prod-safe.
type IdentityDocumentProvider interface {
	IdentityDocumentCreator
	IdentityDocumentValidator
}
