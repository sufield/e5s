//go:build dev

package ports

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// NOTE: This file contains development-only adapter factory interfaces.
// These interfaces are excluded from production builds via build tag.
// Production builds use only CoreAdapterFactory (defined in outbound.go).

// DevelopmentAdapterFactory extends BaseAdapterFactory with development-specific adapters.
// These methods create in-memory implementations for local attestation and registry.
// Production implementations don't need these as they delegate to external SPIRE.
//
// NOTE: This interface is ONLY available in development builds.
// Production code should depend on CoreAdapterFactory only.
type DevelopmentAdapterFactory interface {
	BaseAdapterFactory
	CreateRegistry() IdentityMapperRegistry
	CreateAttestor() WorkloadAttestor
	// CreateDevelopmentServer creates an in-memory server with full control over dependencies.
	CreateDevelopmentServer(ctx context.Context, trustDomain string, trustDomainParser TrustDomainParser, docProvider IdentityDocumentProvider) (IdentityServer, error)
	// CreateDevelopmentAgent creates an in-memory agent with full control over dependencies.
	// Requires all parameters because in-memory implementation manages registry, attestation, and issuance locally.
	CreateDevelopmentAgent(ctx context.Context, spiffeID string, server IdentityServer, registry IdentityMapperRegistry, attestor WorkloadAttestor, parser IdentityCredentialParser, docProvider IdentityDocumentProvider) (Agent, error)
}

// RegistryConfigurator provides registry configuration operations.
// Only used during bootstrap to seed identity mappers (like SPIRE registration entries).
// Production implementations that use external SPIRE Server don't need this.
//
// NOTE: This interface is ONLY available in development builds.
// In production, SPIRE Server manages registration entries via its own API.
type RegistryConfigurator interface {
	SeedRegistry(registry IdentityMapperRegistry, ctx context.Context, mapper *domain.IdentityMapper) error
	SealRegistry(registry IdentityMapperRegistry)
}

// AttestorConfigurator provides attestor configuration operations.
// Only used during bootstrap to register workload UIDs for in-memory attestation.
// Production implementations that use SPIRE Agent don't need this.
//
// NOTE: This interface is ONLY available in development builds.
// In production, SPIRE Agent handles workload attestation via platform attestors.
type AttestorConfigurator interface {
	RegisterWorkloadUID(attestor WorkloadAttestor, uid int, selector string)
}

// AdapterFactory is the composite interface for complete in-memory adapter factory functionality.
// This interface is for development-only implementations that manage all components in-memory.
//
// NOTE: This composite interface is ONLY available in development builds.
// Production code should use CoreAdapterFactory interface (defined in outbound.go).
//
// Design Note: This interface does NOT include CoreAdapterFactory (which includes ProductionAgentFactory)
// because in-memory implementations are inherently development-only and don't make sense for
// production agent creation that requires minimal parameters.
//
// Usage in development:
//   factory := compose.NewInMemoryAdapterFactory()
//   var _ ports.AdapterFactory = factory  // Implements dev interfaces
//
// Usage in production:
//   factory := compose.NewSPIREAdapterFactory(...)
//   var _ ports.CoreAdapterFactory = factory  // Implements production interfaces
type AdapterFactory interface {
	BaseAdapterFactory
	DevelopmentAdapterFactory
	RegistryConfigurator
	AttestorConfigurator
}
