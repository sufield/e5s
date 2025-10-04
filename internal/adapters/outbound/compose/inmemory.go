package compose

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory/attestor"
	"github.com/pocket/hexagon/spire/internal/app/ports"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// InMemoryDeps provides in-memory implementations of all adapters
// This is infrastructure code - not part of core or adapters
type InMemoryDeps struct{}

// NewInMemoryDeps creates dependencies factory for in-memory adapters
func NewInMemoryDeps() *InMemoryDeps {
	return &InMemoryDeps{}
}

func (d *InMemoryDeps) CreateRegistry() ports.IdentityMapperRegistry {
	return inmemory.NewInMemoryRegistry()
}

func (d *InMemoryDeps) CreateTrustDomainParser() ports.TrustDomainParser {
	return inmemory.NewInMemoryTrustDomainParser()
}

func (d *InMemoryDeps) CreateIdentityNamespaceParser() ports.IdentityNamespaceParser {
	return inmemory.NewInMemoryIdentityNamespaceParser()
}

func (d *InMemoryDeps) CreateIdentityDocumentProvider() ports.IdentityDocumentProvider {
	return inmemory.NewInMemoryIdentityDocumentProvider()
}

func (d *InMemoryDeps) CreateServer(ctx context.Context, trustDomain string, trustDomainParser ports.TrustDomainParser, docProvider ports.IdentityDocumentProvider) (ports.Server, error) {
	return inmemory.NewInMemoryServer(ctx, trustDomain, trustDomainParser, docProvider)
}

func (d *InMemoryDeps) CreateAttestor() ports.WorkloadAttestor {
	return attestor.NewUnixWorkloadAttestor()
}

func (d *InMemoryDeps) RegisterWorkloadUID(attestorInterface ports.WorkloadAttestor, uid int, selector string) {
	// Type assert to concrete type for UID registration
	if unixAttestor, ok := attestorInterface.(*attestor.UnixWorkloadAttestor); ok {
		unixAttestor.RegisterUID(uid, selector)
	}
}

func (d *InMemoryDeps) CreateAgent(
	ctx context.Context,
	spiffeID string,
	server ports.Server,
	registry ports.IdentityMapperRegistry,
	attestorInterface ports.WorkloadAttestor,
	parser ports.IdentityNamespaceParser,
	docProvider ports.IdentityDocumentProvider,
) (ports.Agent, error) {
	// Need concrete types for agent creation
	concreteServer, ok := server.(*inmemory.InMemoryServer)
	if !ok {
		panic("expected InMemoryServer")
	}

	return inmemory.NewInMemoryAgent(ctx, spiffeID, concreteServer, registry, attestorInterface, parser, docProvider)
}

// SeedRegistry seeds the registry with an identity mapper (configuration, not runtime)
// This is called only during bootstrap - uses Seed() method on concrete type
func (d *InMemoryDeps) SeedRegistry(registry ports.IdentityMapperRegistry, ctx context.Context, mapper *domain.IdentityMapper) error {
	concreteRegistry, ok := registry.(*inmemory.InMemoryRegistry)
	if !ok {
		return fmt.Errorf("expected InMemoryRegistry for seeding")
	}
	return concreteRegistry.Seed(ctx, mapper)
}

// SealRegistry marks the registry as immutable after seeding
// This prevents any further mutations - registry becomes read-only
func (d *InMemoryDeps) SealRegistry(registry ports.IdentityMapperRegistry) {
	concreteRegistry, ok := registry.(*inmemory.InMemoryRegistry)
	if ok {
		concreteRegistry.Seal()
	}
}
