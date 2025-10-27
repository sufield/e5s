package ports

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/dto"
)

type Agent interface {
	GetIdentity(ctx context.Context) (*domain.IdentityDocument, error)
	FetchIdentityDocument(ctx context.Context, workload *domain.Workload) (*domain.IdentityDocument, error)
	Close() error
}

// ConfigLoader loads runtime configuration.
type ConfigLoader interface {
	Load(ctx context.Context) (*dto.Config, error)
}

type TrustDomainParser interface {
	FromString(ctx context.Context, name string) (*domain.TrustDomain, error)
}

type IdentityCredentialParser interface {
	ParseFromString(ctx context.Context, id string) (*domain.IdentityCredential, error)
	ParseFromPath(ctx context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityCredential, error)
}

type IdentityDocumentValidator interface {
	ValidateIdentityDocument(ctx context.Context, doc *domain.IdentityDocument, expectedID *domain.IdentityCredential) error
}

// BaseAdapterFactory provides core adapter creation for parsing and validation.
// All adapter factories must implement these fundamental operations.
type BaseAdapterFactory interface {
	CreateTrustDomainParser() TrustDomainParser
	CreateIdentityCredentialParser() IdentityCredentialParser
	CreateIdentityDocumentValidator() IdentityDocumentValidator
	Close() error
}

// AgentFactory extends BaseAdapterFactory with agent and identity service creation.
// This interface is used by the application layer to obtain identity operations.
type AgentFactory interface {
	BaseAdapterFactory

	// CreateAgent builds an Agent that can fetch identity documents.
	// The agent connects to SPIRE infrastructure for workload identity operations.
	CreateAgent(ctx context.Context, spiffeID string, parser IdentityCredentialParser) (Agent, error)

	// CreateIdentityService builds an IdentityService that answers "who am I?"
	// in SPIRE-agnostic form (returns ports.Identity, not SPIRE-specific types).
	CreateIdentityService() (IdentityService, error)
}

// AdapterFactory is the complete factory interface consumed by bootstrap.
// It includes all adapter creation capabilities from AgentFactory.
type AdapterFactory interface {
	AgentFactory
}
