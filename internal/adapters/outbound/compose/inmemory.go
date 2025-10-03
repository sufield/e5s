package compose

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory/attestor"
	"github.com/pocket/hexagon/spire/internal/app/ports"
)

// InMemoryDeps provides in-memory implementations of all adapters
// This is infrastructure code - not part of core or adapters
type InMemoryDeps struct{}

// NewInMemoryDeps creates dependencies factory for in-memory adapters
func NewInMemoryDeps() *InMemoryDeps {
	return &InMemoryDeps{}
}

func (d *InMemoryDeps) CreateStore() ports.IdentityStore {
	return inmemory.NewInMemoryStore()
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

func (d *InMemoryDeps) CreateServer(ctx context.Context, trustDomain string, store ports.IdentityStore, trustDomainParser ports.TrustDomainParser, docProvider ports.IdentityDocumentProvider) (ports.Server, error) {
	return inmemory.NewInMemoryServer(ctx, trustDomain, store, trustDomainParser, docProvider)
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
	store ports.IdentityStore,
	attestorInterface ports.WorkloadAttestor,
	parser ports.IdentityNamespaceParser,
	docProvider ports.IdentityDocumentProvider,
) (ports.Agent, error) {
	// Need concrete types for agent creation
	concreteServer, ok := server.(*inmemory.InMemoryServer)
	if !ok {
		panic("expected InMemoryServer")
	}
	concreteStore, ok := store.(*inmemory.InMemoryStore)
	if !ok {
		panic("expected InMemoryStore")
	}

	return inmemory.NewInMemoryAgent(ctx, spiffeID, concreteServer, concreteStore, attestorInterface, parser, docProvider)
}
