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
// Uses configuration struct pattern to reduce parameter count (Go best practice).
func (f *InMemoryAdapterFactory) CreateDevelopmentServer(ctx context.Context, cfg ports.DevelopmentServerConfig) (ports.IdentityServer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid server config: %w", err)
	}
	return inmemory.NewInMemoryServer(ctx, cfg.TrustDomain, cfg.TrustDomainParser, cfg.DocProvider)
}

// CreateServer creates an in-memory server (implements ProductionServerFactory).
// For inmemory, this delegates to CreateDevelopmentServer since the implementation is the same.
func (f *InMemoryAdapterFactory) CreateServer(ctx context.Context, trustDomain string, trustDomainParser ports.TrustDomainParser, docProvider ports.IdentityDocumentProvider) (ports.IdentityServer, error) {
	// Delegate to development server using config struct
	return f.CreateDevelopmentServer(ctx, ports.DevelopmentServerConfig{
		TrustDomain:       trustDomain,
		TrustDomainParser: trustDomainParser,
		DocProvider:       docProvider,
	})
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
// Uses configuration struct pattern to reduce parameter count (Go best practice).
func (f *InMemoryAdapterFactory) CreateDevelopmentAgent(ctx context.Context, cfg ports.DevelopmentAgentConfig) (ports.Agent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid agent config: %w", err)
	}

	// Need concrete types for agent creation
	concreteServer, ok := cfg.Server.(*inmemory.InMemoryServer)
	if !ok {
		return nil, fmt.Errorf("expected *inmemory.InMemoryServer, got %T", cfg.Server)
	}

	return inmemory.NewInMemoryAgent(ctx, cfg.SPIFFEID, concreteServer, cfg.Registry, cfg.Attestor, cfg.Parser, cfg.DocProvider)
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
