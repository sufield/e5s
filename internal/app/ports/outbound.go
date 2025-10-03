package ports

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// ConfigLoader loads application configuration
type ConfigLoader interface {
	Load(ctx context.Context) (*Config, error)
}

// IdentityStore manages identities and their registration
type IdentityStore interface {
	// Register registers a new workload identity
	Register(ctx context.Context, identityNamespace *domain.IdentityNamespace, selector *domain.Selector) error
	// GetIdentity retrieves an identity by identity namespace
	GetIdentity(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*Identity, error)
	// ListIdentities lists all registered identities
	ListIdentities(ctx context.Context) ([]*Identity, error)
}

// IdentityMapperRepository manages identity mappers (identity namespace to selector mappings)
// Identity mappers define authorization policies: which workloads qualify for which identities
// In real SPIRE, this is the server-side registration API
type IdentityMapperRepository interface {
	// CreateMapper creates a new identity mapper
	CreateMapper(ctx context.Context, mapper *domain.IdentityMapper) error
	// FindMatchingMapper finds an identity mapper that matches the given selectors
	// Used during workload attestation to determine which identity namespace to issue
	FindMatchingMapper(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)
	// ListMappers lists all identity mappers (for debugging/admin)
	ListMappers(ctx context.Context) ([]*domain.IdentityMapper, error)
	// DeleteMapper deletes an identity mapper by identity namespace
	DeleteMapper(ctx context.Context, identityNamespace *domain.IdentityNamespace) error
}

// WorkloadAttestor verifies workload identity based on platform-specific attributes
type WorkloadAttestor interface {
	// Attest verifies a workload and returns its selectors
	Attest(ctx context.Context, workload ProcessIdentity) ([]string, error)
}

// NodeAttestor verifies node (agent host) identity and produces attestation data
// In a real SPIRE deployment, this would use platform-specific attestation (AWS IID, TPM, etc.)
// For in-memory walking skeleton, this provides hardcoded/mock attestation
type NodeAttestor interface {
	// AttestNode performs node attestation and returns the attested node with selectors
	// Returns domain.Node with selectors populated and marked as attested
	AttestNode(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*domain.Node, error)
}

// Server represents the identity server functionality
type Server interface {
	// IssueIdentity issues an identity document for a registered workload
	IssueIdentity(ctx context.Context, identityNamespace *domain.IdentityNamespace) (*domain.IdentityDocument, error)
	// GetTrustDomain returns the trust domain
	GetTrustDomain() *domain.TrustDomain
}

// Agent represents the identity agent functionality
type Agent interface {
	// GetIdentity returns the agent's own identity
	GetIdentity(ctx context.Context) (*Identity, error)
	// FetchIdentityDocument fetches an identity document for a workload
	FetchIdentityDocument(ctx context.Context, workload ProcessIdentity) (*Identity, error)
}

// IdentityDocumentValidator validates identity documents using SDK verification logic
// This port abstracts SDK-specific identity document validation (chain-of-trust, expiration, etc.)
type IdentityDocumentValidator interface {
	// Validate verifies an identity document is valid and matches the expected identity namespace
	// This may use go-spiffe SDK's ParseAndVerify or similar validation logic
	Validate(ctx context.Context, doc *domain.IdentityDocument, expectedID *domain.IdentityNamespace) error
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

// IdentityNamespaceParser parses and validates identity namespace strings
// This port abstracts SDK-specific identity namespace parsing (e.g., go-spiffe's spiffeid.FromString)
// to avoid duplicating SDK logic in the domain layer.
//
// Design Note: The go-spiffe SDK provides mature, battle-tested parsing/validation
// via spiffeid.FromString and spiffeid.FromPath. By using this port:
// - Real implementation can use SDK for proper validation (scheme, host format, path normalization)
// - In-memory implementation can use simple string parsing for walking skeleton
// - Domain remains SDK-agnostic (only holds parsed data, doesn't parse)
type IdentityNamespaceParser interface {
	// ParseFromString parses an identity namespace from a URI string (e.g., "spiffe://example.org/host")
	// Validates scheme, extracts trust domain and path, returns domain.IdentityNamespace
	ParseFromString(ctx context.Context, id string) (*domain.IdentityNamespace, error)
	// ParseFromPath creates an identity namespace from trust domain and path components
	// Ensures path starts with "/", formats as spiffe://<td><path>
	ParseFromPath(ctx context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityNamespace, error)
}

// IdentityDocumentProvider creates and manages identity documents
// This port abstracts SDK-specific identity document creation/validation (e.g., go-spiffe's x509svid package)
// to avoid duplicating SDK logic in the domain layer.
//
// Design Note: The go-spiffe SDK provides mature IdentityDocument handling:
// - x509svid.ParseX509SVID(certBytes, keyBytes) for DER parsing
// - x509svid.Verify(cert, chain, bundle) for chain-of-trust validation
// - Proper crypto.Signer handling for private keys
// By using this port:
// - Real implementation can use SDK for proper document handling
// - In-memory implementation can generate simple test documents
// - Domain remains crypto-agnostic (only holds result data)
//
// Naming: "IdentityDocumentProvider" is more inclusive than "CertificateProvider" (encompasses
// both X.509 and JWT formats) while remaining self-explanatory and domain-focused.
type IdentityDocumentProvider interface {
	// CreateX509IdentityDocument creates an X.509 identity document with certificate and private key
	// Generates certificate signed by CA, extracts expiration, returns domain.IdentityDocument
	// In real implementation: uses SDK's x509svid.Parse or manual x509.CreateCertificate
	CreateX509IdentityDocument(ctx context.Context, identityNamespace *domain.IdentityNamespace, caCert interface{}, caKey interface{}) (*domain.IdentityDocument, error)
	// ValidateIdentityDocument performs full identity document validation (time, chain-of-trust, signature)
	// Checks identity namespace match, expiration, and optionally bundle verification
	// Returns domain sentinel errors (ErrIdentityDocumentExpired, ErrIdentityDocumentInvalid, ErrIdentityDocumentMismatch)
	ValidateIdentityDocument(ctx context.Context, doc *domain.IdentityDocument, expectedID *domain.IdentityNamespace) error
}
