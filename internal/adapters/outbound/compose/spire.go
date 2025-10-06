package compose

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory/attestor"
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
// For production, we still use in-memory registry as it's seeded at startup
func (f *SPIREAdapterFactory) CreateRegistry() ports.IdentityMapperRegistry {
	return inmemory.NewInMemoryRegistry()
}

// CreateTrustDomainParser creates a trust domain parser
// Uses in-memory parser which wraps go-spiffe SDK
func (f *SPIREAdapterFactory) CreateTrustDomainParser() ports.TrustDomainParser {
	return inmemory.NewInMemoryTrustDomainParser()
}

// CreateIdentityNamespaceParser creates an identity namespace parser
// Uses in-memory parser which wraps go-spiffe SDK
func (f *SPIREAdapterFactory) CreateIdentityNamespaceParser() ports.IdentityNamespaceParser {
	return inmemory.NewInMemoryIdentityNamespaceParser()
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
) (ports.Server, error) {
	return spire.NewServer(ctx, f.client, trustDomain, trustDomainParser)
}

// CreateAttestor creates a workload attestor
// Uses Unix workload attestor for process-based attestation
func (f *SPIREAdapterFactory) CreateAttestor() ports.WorkloadAttestor {
	return attestor.NewUnixWorkloadAttestor()
}

// RegisterWorkloadUID registers a workload UID with the attestor
func (f *SPIREAdapterFactory) RegisterWorkloadUID(attestorInterface ports.WorkloadAttestor, uid int, selector string) {
	// Type assert to concrete type for UID registration
	if unixAttestor, ok := attestorInterface.(*attestor.UnixWorkloadAttestor); ok {
		unixAttestor.RegisterUID(uid, selector)
	}
}

// CreateAgent creates a SPIRE-backed agent
func (f *SPIREAdapterFactory) CreateAgent(
	ctx context.Context,
	spiffeID string,
	server ports.Server,
	registry ports.IdentityMapperRegistry,
	attestorInterface ports.WorkloadAttestor,
	parser ports.IdentityNamespaceParser,
	docProvider ports.IdentityDocumentProvider,
) (ports.Agent, error) {
	return spire.NewAgent(ctx, f.client, spiffeID, registry, attestorInterface, parser)
}

// SeedRegistry seeds the registry with an identity mapper
func (f *SPIREAdapterFactory) SeedRegistry(registry ports.IdentityMapperRegistry, ctx context.Context, mapper *domain.IdentityMapper) error {
	concreteRegistry, ok := registry.(*inmemory.InMemoryRegistry)
	if !ok {
		return fmt.Errorf("expected InMemoryRegistry for seeding")
	}
	return concreteRegistry.Seed(ctx, mapper)
}

// SealRegistry marks the registry as immutable after seeding
func (f *SPIREAdapterFactory) SealRegistry(registry ports.IdentityMapperRegistry) {
	concreteRegistry, ok := registry.(*inmemory.InMemoryRegistry)
	if ok {
		concreteRegistry.Seal()
	}
}

// Close closes the SPIRE client connection
func (f *SPIREAdapterFactory) Close() error {
	if f.client != nil {
		return f.client.Close()
	}
	return nil
}

var _ ports.AdapterFactory = (*SPIREAdapterFactory)(nil)
