package compose

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// SPIREAdapterFactory provides production SPIRE implementations of adapters
// Uses external SPIRE infrastructure instead of in-memory implementations
type SPIREAdapterFactory struct {
	config *spire.Config
	client *spire.SPIREClient
}

// NewSPIREAdapterFactory creates the factory for production SPIRE adapters
func NewSPIREAdapterFactory(ctx context.Context, cfg *spire.Config) (*SPIREAdapterFactory, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Create SPIRE Workload API client
	client, err := spire.NewSPIREClient(ctx, *cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE client: %w", err)
	}

	return &SPIREAdapterFactory{
		config: cfg,
		client: client,
	}, nil
}

// CreateRegistry creates an identity mapper registry
// PRODUCTION NOTE: In production mode with SPIRE, the registry is managed by SPIRE Server.
// This method is not used when using ProductionAgent.
// Returns nil to indicate registry is external.
func (f *SPIREAdapterFactory) CreateRegistry() ports.IdentityMapperRegistry {
	// Production uses SPIRE Server's registration entries, not in-memory registry
	return nil
}

// CreateTrustDomainParser creates a trust domain parser
// Uses in-memory parser which wraps go-spiffe SDK
func (f *SPIREAdapterFactory) CreateTrustDomainParser() ports.TrustDomainParser {
	return inmemory.NewInMemoryTrustDomainParser()
}

// CreateIdentityCredentialParser creates an identity credential parser
// Uses in-memory parser which wraps go-spiffe SDK
func (f *SPIREAdapterFactory) CreateIdentityCredentialParser() ports.IdentityCredentialParser {
	return inmemory.NewInMemoryIdentityCredentialParser()
}

// CreateIdentityDocumentProvider creates an identity document provider
// For production, document creation is handled by SPIRE Server
// We use in-memory provider for validation logic
func (f *SPIREAdapterFactory) CreateIdentityDocumentProvider() ports.IdentityDocumentProvider {
	return inmemory.NewInMemoryIdentityDocumentProvider()
}

// CreateServer creates a SPIRE-backed server
func (f *SPIREAdapterFactory) CreateServer(
	ctx context.Context,
	trustDomain string,
	trustDomainParser ports.TrustDomainParser,
	docProvider ports.IdentityDocumentProvider,
) (ports.IdentityServer, error) {
	return spire.NewServer(ctx, f.client, trustDomain, trustDomainParser)
}

// CreateAttestor creates a workload attestor
// PRODUCTION NOTE: In production mode with SPIRE, attestation is handled by SPIRE Agent.
// This method is not used when using ProductionAgent.
// Returns nil to indicate attestation is external.
func (f *SPIREAdapterFactory) CreateAttestor() ports.WorkloadAttestor {
	// Production uses SPIRE Agent for workload attestation, not local attestor
	return nil
}

// RegisterWorkloadUID registers a workload UID with the attestor
// PRODUCTION NOTE: Not used in production mode - SPIRE Server manages registration entries
func (f *SPIREAdapterFactory) RegisterWorkloadUID(attestorInterface ports.WorkloadAttestor, uid int, selector string) {
	// No-op in production mode - SPIRE Server handles workload registration
}

// CreateAgent creates a SPIRE-backed production agent
// This agent fully delegates to external SPIRE for registry and attestation
func (f *SPIREAdapterFactory) CreateAgent(
	ctx context.Context,
	spiffeID string,
	server ports.IdentityServer,
	registry ports.IdentityMapperRegistry,
	attestorInterface ports.WorkloadAttestor,
	parser ports.IdentityCredentialParser,
	docProvider ports.IdentityDocumentProvider,
) (ports.Agent, error) {
	// Use Agent which delegates everything to SPIRE
	// Registry and attestor parameters are ignored (they're nil in production)
	return spire.NewAgent(ctx, f.client, spiffeID, parser)
}

// SeedRegistry seeds the registry with an identity mapper
// PRODUCTION NOTE: Not used in production - SPIRE Server manages registration entries
// Registration entries should be created via SPIRE Server API or CLI
func (f *SPIREAdapterFactory) SeedRegistry(registry ports.IdentityMapperRegistry, ctx context.Context, mapper *domain.IdentityMapper) error {
	// No-op in production - registry is managed by SPIRE Server
	// Registration entries are created using: spire-server entry create
	return nil
}

// SealRegistry marks the registry as immutable after seeding
// PRODUCTION NOTE: Not used in production - SPIRE Server manages registry lifecycle
func (f *SPIREAdapterFactory) SealRegistry(registry ports.IdentityMapperRegistry) {
	// No-op in production - SPIRE Server manages registry
}

// Close closes the SPIRE client connection
func (f *SPIREAdapterFactory) Close() error {
	if f.client != nil {
		return f.client.Close()
	}
	return nil
}

var _ ports.AdapterFactory = (*SPIREAdapterFactory)(nil)
