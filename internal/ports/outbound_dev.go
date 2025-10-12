//go:build dev

package ports

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// NOTE: This file contains development-only adapter factory interfaces.
// These interfaces are excluded from production builds via build tag.
// Production builds use only CoreAdapterFactory (defined in outbound.go).

// DevelopmentServerConfig holds configuration for creating a development server.
type DevelopmentServerConfig struct {
	TrustDomain       string
	TrustDomainParser TrustDomainParser
	DocProvider       IdentityDocumentProvider
}

// Validate checks that all required fields are set.
func (c *DevelopmentServerConfig) Validate() error {
	if c.TrustDomain == "" {
		return fmt.Errorf("trust domain cannot be empty")
	}
	if c.TrustDomainParser == nil {
		return fmt.Errorf("trust domain parser cannot be nil")
	}
	if c.DocProvider == nil {
		return fmt.Errorf("identity document provider cannot be nil")
	}
	return nil
}

// DevelopmentAgentConfig holds configuration for creating a development agent.
type DevelopmentAgentConfig struct {
	SPIFFEID    string
	Server      IdentityServer
	Registry    IdentityMapperRegistry
	Attestor    WorkloadAttestor
	Parser      IdentityCredentialParser
	DocProvider IdentityDocumentProvider
}

// Validate checks that all required fields are set.
func (c *DevelopmentAgentConfig) Validate() error {
	if c.SPIFFEID == "" {
		return fmt.Errorf("SPIFFE ID cannot be empty")
	}
	if c.Server == nil {
		return fmt.Errorf("server cannot be nil")
	}
	if c.Registry == nil {
		return fmt.Errorf("registry cannot be nil")
	}
	if c.Attestor == nil {
		return fmt.Errorf("attestor cannot be nil")
	}
	if c.Parser == nil {
		return fmt.Errorf("parser cannot be nil")
	}
	if c.DocProvider == nil {
		return fmt.Errorf("identity document provider cannot be nil")
	}
	return nil
}

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
	// Uses configuration struct pattern to reduce parameter count (Go best practice).
	CreateDevelopmentServer(ctx context.Context, cfg DevelopmentServerConfig) (IdentityServer, error)
	// CreateDevelopmentAgent creates an in-memory agent with full control over dependencies.
	// Uses configuration struct pattern to reduce parameter count (Go best practice).
	// Requires all config fields because in-memory implementation manages registry, attestation, and issuance locally.
	CreateDevelopmentAgent(ctx context.Context, cfg DevelopmentAgentConfig) (Agent, error)
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
	// RegisterWorkloadUID registers a UID with the attestor if it supports dev-mode registration.
	// Returns true if registration succeeded, false if the attestor doesn't support registration.
	// Callers can assert the boolean in tests to ensure registration worked.
	RegisterWorkloadUID(attestor WorkloadAttestor, uid int, selector string) bool
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
