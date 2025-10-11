//go:build dev

package compose

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory/attestor"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryAdapterFactory provides in-memory implementations of all adapters
// Implements the AdapterFactory port for in-memory/walking skeleton mode
//
// NOTE: This file is excluded from production builds via build tag.
// Production deployments use SPIREAdapterFactory (defined in spire.go) instead.
type InMemoryAdapterFactory struct{}

// NewInMemoryAdapterFactory creates the factory for in-memory adapters
func NewInMemoryAdapterFactory() *InMemoryAdapterFactory {
	return &InMemoryAdapterFactory{}
}

func (f *InMemoryAdapterFactory) CreateRegistry() ports.IdentityMapperRegistry {
	return inmemory.NewInMemoryRegistry()
}

func (f *InMemoryAdapterFactory) CreateTrustDomainParser() ports.TrustDomainParser {
	return inmemory.NewInMemoryTrustDomainParser()
}

func (f *InMemoryAdapterFactory) CreateIdentityCredentialParser() ports.IdentityCredentialParser {
	return inmemory.NewInMemoryIdentityCredentialParser()
}

func (f *InMemoryAdapterFactory) CreateIdentityDocumentProvider() ports.IdentityDocumentProvider {
	return inmemory.NewInMemoryIdentityDocumentProvider()
}

func (f *InMemoryAdapterFactory) CreateTrustBundleProvider(server ports.IdentityServer) ports.TrustBundleProvider {
	// Extract CA certificate from server for bundle
	caCert := server.GetCA()
	if caCert == nil {
		// Return empty provider if CA not initialized
		return inmemory.NewInMemoryTrustBundleProvider(nil)
	}
	// Support multi-CA by wrapping single CA in slice (aligned with SDK bundle format)
	return inmemory.NewInMemoryTrustBundleProvider([]*x509.Certificate{caCert})
}

// CreateDevelopmentServer creates an in-memory server for development/testing.
// Unlike production, this implementation needs full control over document provider
// because it manages certificate issuance locally.
func (f *InMemoryAdapterFactory) CreateDevelopmentServer(ctx context.Context, trustDomain string, trustDomainParser ports.TrustDomainParser, docProvider ports.IdentityDocumentProvider) (ports.IdentityServer, error) {
	return inmemory.NewInMemoryServer(ctx, trustDomain, trustDomainParser, docProvider)
}

// CreateServer creates an in-memory server (implements ProductionServerFactory).
// For inmemory, this delegates to CreateDevelopmentServer since the implementation is the same.
func (f *InMemoryAdapterFactory) CreateServer(ctx context.Context, trustDomain string, trustDomainParser ports.TrustDomainParser, docProvider ports.IdentityDocumentProvider) (ports.IdentityServer, error) {
	return f.CreateDevelopmentServer(ctx, trustDomain, trustDomainParser, docProvider)
}

func (f *InMemoryAdapterFactory) CreateAttestor() ports.WorkloadAttestor {
	return attestor.NewUnixWorkloadAttestor()
}

func (f *InMemoryAdapterFactory) RegisterWorkloadUID(attestorInterface ports.WorkloadAttestor, uid int, selector string) {
	// Type assert to concrete type for UID registration
	if unixAttestor, ok := attestorInterface.(*attestor.UnixWorkloadAttestor); ok {
		unixAttestor.RegisterUID(uid, selector)
	}
}

// CreateDevelopmentAgent creates an in-memory agent for development/testing.
// Unlike production, this implementation requires all dependencies because it
// manages registry, attestation, and identity issuance locally.
func (f *InMemoryAdapterFactory) CreateDevelopmentAgent(
	ctx context.Context,
	spiffeID string,
	server ports.IdentityServer,
	registry ports.IdentityMapperRegistry,
	attestorInterface ports.WorkloadAttestor,
	parser ports.IdentityCredentialParser,
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
func (f *InMemoryAdapterFactory) SeedRegistry(registry ports.IdentityMapperRegistry, ctx context.Context, mapper *domain.IdentityMapper) error {
	concreteRegistry, ok := registry.(*inmemory.InMemoryRegistry)
	if !ok {
		return fmt.Errorf("expected InMemoryRegistry for seeding")
	}
	return concreteRegistry.Seed(ctx, mapper)
}

// SealRegistry marks the registry as immutable after seeding
// This prevents any further mutations - registry becomes read-only
func (f *InMemoryAdapterFactory) SealRegistry(registry ports.IdentityMapperRegistry) {
	concreteRegistry, ok := registry.(*inmemory.InMemoryRegistry)
	if ok {
		concreteRegistry.Seal()
	}
}

var _ ports.AdapterFactory = (*InMemoryAdapterFactory)(nil)
