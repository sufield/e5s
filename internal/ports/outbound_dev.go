//go:build dev

package ports

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/dto"
)

// ConfigLoader loads runtime configuration (dev-only).
type ConfigLoader interface {
	Load(ctx context.Context) (*dto.Config, error)
}

// IdentityMapperRegistry provides read-only access to the identity mapper registry seeded at startup.
// Dev-only: in production, SPIRE Server manages registration entries.
//
// Error Contract:
// - FindBySelectors: domain.ErrNoMatchingMapper if no mapper matches
// - FindBySelectors: domain.ErrInvalidSelectors if selectors are nil/empty
// - ListAll:         domain.ErrRegistryEmpty if no mappers seeded
//
// Implementations should respect ctx cancellation where applicable.
type IdentityMapperRegistry interface {
	// FindBySelectors finds an identity mapper matching the given selectors.
	// AND semantics: all mapper selectors must be present in the discovered selectors.
	// Returns the first matching mapper (deterministic order depends on registry seeding).
	FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)

	// ListAll returns all seeded identity mappers (for debugging/admin).
	ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}

// WorkloadAttestor verifies workload identity based on platform-specific attributes.
// Dev-only: in production, SPIRE Agent performs attestation automatically.
//
// Error Contract:
// - domain.ErrWorkloadAttestationFailed if attestation fails
// - domain.ErrInvalidProcessIdentity   if workload info is invalid
// - domain.ErrNoAttestationData        if no selectors can be generated
//
// Implementations should respect ctx cancellation where applicable.
type WorkloadAttestor interface {
	// Attest verifies a workload and returns its selectors.
	// Selectors must be formatted as "type:key:value" (e.g., "unix:uid:1000", "k8s:namespace:prod").
	Attest(ctx context.Context, workload *domain.Workload) ([]string, error)
}

// IdentityServer represents identity server functionality for in-memory/dev mode only.
// Dev-only: in production, SPIRE Server runs as external infrastructure.
//
// Error Contract:
// - IssueIdentity: domain.ErrIdentityDocumentInvalid if identity credential invalid
// - IssueIdentity: domain.ErrServerUnavailable if server unavailable
// - IssueIdentity: domain.ErrCANotInitialized if CA not initialized
// - GetTrustDomain: never returns error (returns nil if not initialized)
// - GetCACertPEM:  returns empty slice if CA not initialized
//
// Implementations should respect ctx cancellation where applicable.
type IdentityServer interface {
	// IssueIdentity issues an identity document for an identity credential.
	// Generates X.509 certificate signed by CA with identity credential in URI SAN.
	IssueIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) (*domain.IdentityDocument, error)

	// GetTrustDomain returns the trust domain this server manages.
	GetTrustDomain() *domain.TrustDomain

	// GetCACertPEM returns the CA certificate as PEM bytes (root of trust).
	// Returns empty slice if CA not initialized - caller must check.
	GetCACertPEM() []byte
}

// IdentityDocumentCreator creates identity documents (X.509 SVIDs).
// Dev-only: in production, SPIRE Server handles creation.
//
// Error Contract:
// - CreateX509IdentityDocument: domain.ErrIdentityDocumentInvalid for invalid inputs
// - CreateX509IdentityDocument: domain.ErrCANotInitialized if CA not available
//
// Implementations should respect ctx cancellation where applicable.
type IdentityDocumentCreator interface {
	// CreateX509IdentityDocument creates an X.509 identity document with certificate and private key.
	// Generates certificate signed by CA, extracts expiration, returns domain.IdentityDocument.
	//
	// Note: caCert and caKey are interface{} to avoid leaking crypto/x509 types into ports.
	// Implementations cast to appropriate types (*x509.Certificate, crypto.Signer).
	CreateX509IdentityDocument(ctx context.Context, identityCredential *domain.IdentityCredential, caCert interface{}, caKey interface{}) (*domain.IdentityDocument, error)
}

// IdentityDocumentProvider combines creation and validation of identity documents.
// Dev-only: in production, use IdentityDocumentValidator (from outbound.go) which is prod-safe.
type IdentityDocumentProvider interface {
	IdentityDocumentCreator
	IdentityDocumentValidator
}

// TrustBundleProvider provides trust bundles for validation (dev-only).
type TrustBundleProvider interface {
	GetBundle(ctx context.Context, trustDomain *domain.TrustDomain) ([]byte, error)
	GetBundleForIdentity(ctx context.Context, identityCredential *domain.IdentityCredential) ([]byte, error)
}
