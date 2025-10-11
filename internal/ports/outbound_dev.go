//go:build dev

package ports

import (
	"context"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// NOTE: This file contains development-only adapter factory interfaces.
// These interfaces are excluded from production builds via build tag.
// Production builds use only CoreAdapterFactory (defined in outbound.go).

// DevelopmentAdapterFactory extends CoreAdapterFactory with development-specific adapters.
// These methods create in-memory implementations for local attestation and registry.
// Production implementations don't need these as they delegate to external SPIRE.
//
// NOTE: This interface is ONLY available in development builds.
// Production code should depend on CoreAdapterFactory only.
type DevelopmentAdapterFactory interface {
	CoreAdapterFactory
	CreateRegistry() IdentityMapperRegistry
	CreateAttestor() WorkloadAttestor
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

// AdapterFactory is the composite interface for complete adapter factory functionality.
// Development implementations provide all capabilities.
// Production implementations only implement CoreAdapterFactory.
//
// NOTE: This composite interface is ONLY available in development builds.
// Production code should use CoreAdapterFactory interface.
//
// Usage in development:
//   factory := compose.NewInMemoryAdapterFactory()
//   var _ ports.AdapterFactory = factory  // Implements all 4 interfaces
//
// Usage in production:
//   factory := compose.NewSPIREAdapterFactory(...)
//   var _ ports.CoreAdapterFactory = factory  // Implements only core
type AdapterFactory interface {
	CoreAdapterFactory
	DevelopmentAdapterFactory
	RegistryConfigurator
	AttestorConfigurator
}
