//go:build dev

package ports

import (
	"context"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// IdentityMapperRegistry provides read-only access to the identity mapper registry seeded at startup.
// This interface is only available in development builds for in-memory implementations.
//
// In production deployments, SPIRE Server manages registration entries. Workloads only fetch
// their identity via Workload API - no local registry or selector matching is needed.
//
// Error Contract:
// - FindBySelectors returns domain.ErrNoMatchingMapper if no mapper matches
// - FindBySelectors returns domain.ErrInvalidSelectors if selectors are nil/empty
// - ListAll returns domain.ErrRegistryEmpty if no mappers seeded
type IdentityMapperRegistry interface {
	// FindBySelectors finds an identity mapper matching the given selectors (AND logic)
	// This is the core runtime operation: selectors → identity credential mapping
	// All mapper selectors must be present in discovered selectors for a match
	FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error)

	// ListAll returns all seeded identity mappers (for debugging/admin)
	ListAll(ctx context.Context) ([]*domain.IdentityMapper, error)
}

// WorkloadAttestor verifies workload identity based on platform-specific attributes.
// This interface is only available in development builds for in-memory attestation.
//
// In production deployments, SPIRE Agent performs attestation. Workloads connect
// to the agent's Unix socket, and the agent extracts credentials and attests automatically.
//
// Error Contract:
// - Returns domain.ErrWorkloadAttestationFailed if attestation fails
// - Returns domain.ErrInvalidProcessIdentity if workload info is invalid
// - Returns domain.ErrNoAttestationData if no selectors can be generated
type WorkloadAttestor interface {
	// Attest verifies a workload and returns its selectors
	// Selectors format: "type:value" (e.g., "unix:uid:1000", "k8s:namespace:prod")
	Attest(ctx context.Context, workload ProcessIdentity) ([]string, error)
}
