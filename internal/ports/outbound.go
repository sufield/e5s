package ports

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/domain"
)

type ConfigLoader interface {
	Load(ctx context.Context) (*Config, error)
}

type Agent interface {
	GetIdentity(ctx context.Context) (*Identity, error)
	FetchIdentityDocument(ctx context.Context, workload ProcessIdentity) (*domain.IdentityDocument, error)
	Close() error
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

type BaseAdapterFactory interface {
	CreateTrustDomainParser() TrustDomainParser
	CreateIdentityCredentialParser() IdentityCredentialParser
	CreateIdentityDocumentValidator() IdentityDocumentValidator
}

type AgentFactory interface {
	BaseAdapterFactory
	CreateAgent(ctx context.Context, spiffeID string, parser IdentityCredentialParser) (Agent, error)
}

type AdapterFactory interface {
	BaseAdapterFactory
	AgentFactory
}
