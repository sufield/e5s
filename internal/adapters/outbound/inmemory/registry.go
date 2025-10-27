//go:build dev

package inmemory

import (
	"context"
	"fmt"
	"sort"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// InMemoryRegistry is a deterministic fake registry seeded at startup with identity mappings (dev-only)
// It implements the ports.IdentityMapperRegistry interface for runtime read operations
// Seeding is done via internal seed() method called only during bootstrap (internal use by factory only)
// No concurrency support needed - all access is sequential in test/dev mode
type InMemoryRegistry struct {
	mappers map[string]*domain.IdentityMapper // identityCredential.String() → IdentityMapper
	sealed  bool                              // Prevents modifications after bootstrap
}

// NewInMemoryRegistry creates a new unsealed in-memory registry
// Registry must be sealed after seeding to become immutable
// NOTE: This is package-private. External code should use NewSeededRegistry.
func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{
		mappers: make(map[string]*domain.IdentityMapper),
		sealed:  false,
	}
}

// NewSeededRegistry creates a registry, seeds it with provided mappers, and seals it.
// This is the public constructor for external packages (like compose factory).
// The registry is immutable after creation - no external code can mutate it.
func NewSeededRegistry(ctx context.Context, mappers []*domain.IdentityMapper) (*InMemoryRegistry, error) {
	reg := NewInMemoryRegistry()
	if err := reg.seedMany(ctx, mappers); err != nil {
		return nil, err
	}
	reg.seal()
	return reg, nil
}

// seed adds an identity mapper to the registry (internal use by factory only)
// This is NOT part of the IdentityMapperRegistry interface
// Package-private: only code in the inmemory package can call this during composition root setup
func (r *InMemoryRegistry) seed(ctx context.Context, mapper *domain.IdentityMapper) error {
	if r.sealed {
		return fmt.Errorf("inmemory: %w", domain.ErrRegistrySealed)
	}

	// Guard against nil input to avoid panics
	if mapper == nil || mapper.IdentityCredential() == nil {
		return fmt.Errorf("inmemory: invalid identity mapper (nil)")
	}

	idStr := mapper.IdentityCredential().String()
	if _, exists := r.mappers[idStr]; exists {
		return fmt.Errorf("inmemory: identity mapper for %s already exists", idStr)
	}

	r.mappers[idStr] = mapper
	return nil
}

// seedMany adds multiple identity mappers to the registry (internal use by factory only)
// Convenience method for bulk seeding during composition root setup
func (r *InMemoryRegistry) seedMany(ctx context.Context, mappers []*domain.IdentityMapper) error {
	for _, mapper := range mappers {
		if err := r.seed(ctx, mapper); err != nil {
			return err
		}
	}
	return nil
}

// seal marks the registry as immutable after bootstrap (internal use by factory only)
// Once sealed, no further seeding is allowed
// Package-private: external code cannot unseal or re-seal
func (r *InMemoryRegistry) seal() {
	r.sealed = true
}

// FindBySelectors finds an identity mapper matching the given selectors (READ-ONLY)
// Uses AND logic: ALL mapper selectors must be present in the discovered selectors
// This implements the core runtime operation: selectors → identity mapping
// Returns mappers in deterministic order (sorted by identity credential string)
func (r *InMemoryRegistry) FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error) {
	// Validate input
	if selectors == nil || len(selectors.All()) == 0 {
		return nil, fmt.Errorf("inmemory: %w: selectors are nil or empty", domain.ErrInvalidSelectors)
	}

	// Sort keys for deterministic iteration order (avoid flaky tests)
	ids := make([]string, 0, len(r.mappers))
	for id := range r.mappers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Match selectors against all mappers in deterministic order
	for _, id := range ids {
		mapper := r.mappers[id]
		if mapper.MatchesSelectors(selectors) {
			return mapper, nil
		}
	}

	return nil, fmt.Errorf("inmemory: %w: no mapper matches selectors %v", domain.ErrNoMatchingMapper, selectors)
}

// ListAll returns all seeded identity mappers in deterministic order (READ-ONLY, for debugging/admin)
// Returns mappers sorted by identity credential string for stable ordering
func (r *InMemoryRegistry) ListAll(ctx context.Context) ([]*domain.IdentityMapper, error) {
	if len(r.mappers) == 0 {
		return nil, fmt.Errorf("inmemory: %w: no mappers have been seeded", domain.ErrRegistryEmpty)
	}

	// Sort keys for deterministic iteration order (stable assertions in tests)
	ids := make([]string, 0, len(r.mappers))
	for id := range r.mappers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Build result in sorted order
	mappers := make([]*domain.IdentityMapper, 0, len(ids))
	for _, id := range ids {
		mappers = append(mappers, r.mappers[id])
	}

	return mappers, nil
}

// Ensure InMemoryRegistry implements IdentityMapperRegistry
var _ ports.IdentityMapperRegistry = (*InMemoryRegistry)(nil)
