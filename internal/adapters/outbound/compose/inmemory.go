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
	if server == nil {
		return inmemory.NewInMemoryTrustBundleProvider(nil)
	}

	// Prefer a full bundle if the server exposes it (dev/prod parity helper).
	type caBundler interface {
		GetCABundle() []*x509.Certificate
	}

	var bundle []*x509.Certificate
	if s, ok := server.(caBundler); ok {
		b := s.GetCABundle()
		if len(b) > 0 {
			// Defensive copy to prevent mutations
			bundle = make([]*x509.Certificate, len(b))
			copy(bundle, b)
			return inmemory.NewInMemoryTrustBundleProvider(bundle)
		}
	}

	// Fallback to single CA
	if ca := server.GetCA(); ca != nil {
		// Defensive copy (single element slice)
		bundle = make([]*x509.Certificate, 1)
		bundle[0] = ca // Pointer copy is fine for immutable certs, but slice is copied
		return inmemory.NewInMemoryTrustBundleProvider(bundle)
	}
	return inmemory.NewInMemoryTrustBundleProvider(nil)
}

// CreateDevelopmentServer creates an in-memory server for development/testing.
// Unlike production, this implementation needs full control over document provider
// because it manages certificate issuance locally.
func (f *InMemoryAdapterFactory) CreateDevelopmentServer(ctx context.Context, trustDomain string, trustDomainParser ports.TrustDomainParser, docProvider ports.IdentityDocumentProvider) (ports.IdentityServer, error) {
	if trustDomain == "" {
		return nil, fmt.Errorf("trust domain cannot be empty")
	}
	if trustDomainParser == nil {
		return nil, fmt.Errorf("trust domain parser cannot be nil")
	}
	if docProvider == nil {
		return nil, fmt.Errorf("identity document provider cannot be nil")
	}
	return inmemory.NewInMemoryServer(ctx, trustDomain, trustDomainParser, docProvider)
}

// CreateServer creates an in-memory server (implements ProductionServerFactory).
// For inmemory, this delegates to CreateDevelopmentServer since the implementation is the same.
func (f *InMemoryAdapterFactory) CreateServer(ctx context.Context, trustDomain string, trustDomainParser ports.TrustDomainParser, docProvider ports.IdentityDocumentProvider) (ports.IdentityServer, error) {
	// Same validation as development server
	return f.CreateDevelopmentServer(ctx, trustDomain, trustDomainParser, docProvider)
}

func (f *InMemoryAdapterFactory) CreateAttestor() ports.WorkloadAttestor {
	return attestor.NewUnixWorkloadAttestor()
}

// uidRegistrar is a dev-only interface for registering UIDs in test/dev attestors.
// This interface prevents brittle downcasts and makes registration success explicit.
type uidRegistrar interface {
	RegisterUID(uid int, selector string)
}

// RegisterWorkloadUID registers a UID with the attestor if it supports dev-mode registration.
// Returns true if registration succeeded, false if the attestor doesn't support registration.
// Callers can assert the boolean in tests to ensure registration worked.
func (f *InMemoryAdapterFactory) RegisterWorkloadUID(attestorInterface ports.WorkloadAttestor, uid int, selector string) bool {
	if r, ok := attestorInterface.(uidRegistrar); ok {
		r.RegisterUID(uid, selector)
		return true
	}
	return false
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
	if spiffeID == "" {
		return nil, fmt.Errorf("SPIFFE ID cannot be empty")
	}
	if server == nil || registry == nil || attestorInterface == nil || parser == nil || docProvider == nil {
		return nil, fmt.Errorf("all arguments must be non-nil")
	}

	// Need concrete types for agent creation
	concreteServer, ok := server.(*inmemory.InMemoryServer)
	if !ok {
		return nil, fmt.Errorf("expected *inmemory.InMemoryServer, got %T", server)
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
