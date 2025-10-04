package inmemory

import (
	"context"
	"fmt"
	"sync"

	"github.com/pocket/hexagon/spire/internal/app/ports"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// InMemoryRegistry is a read-only registry seeded at startup with identity mappings
// It implements the ports.IdentityMapperRegistry interface for runtime read operations
// Seeding is done via internal seed() method called only during bootstrap
type InMemoryRegistry struct {
	mu       sync.RWMutex
	mappers  map[string]*domain.IdentityMapper // identityNamespace.String() → IdentityMapper
	sealed   bool                               // Prevents modifications after bootstrap
}

// NewInMemoryRegistry creates a new unsealed in-memory registry
// Registry must be sealed after seeding to become immutable
func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{
		mappers: make(map[string]*domain.IdentityMapper),
		sealed:  false,
	}
}

// Seed adds an identity mapper to the registry (INTERNAL - used only during bootstrap)
// This is NOT part of the IdentityMapperRegistry interface - it's only for composition root seeding
// Do not call this method from application services - it's infrastructure/configuration only
func (r *InMemoryRegistry) Seed(ctx context.Context, mapper *domain.IdentityMapper) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sealed {
		return fmt.Errorf("registry is sealed, cannot seed after bootstrap")
	}

	idStr := mapper.IdentityNamespace().String()
	if _, exists := r.mappers[idStr]; exists {
		return fmt.Errorf("identity mapper for %s already exists", idStr)
	}

	r.mappers[idStr] = mapper
	return nil
}

// Seal marks the registry as immutable after bootstrap
// Once sealed, no further seeding is allowed
func (r *InMemoryRegistry) Seal() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sealed = true
}

// FindBySelectors finds an identity mapper matching the given selectors (READ-ONLY)
// Uses AND logic: ALL mapper selectors must be present in the discovered selectors
// This implements the core runtime operation: selectors → identity mapping
func (r *InMemoryRegistry) FindBySelectors(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Match selectors against all mappers
	for _, mapper := range r.mappers {
		if mapper.MatchesSelectors(selectors) {
			return mapper, nil
		}
	}

	return nil, domain.ErrNoMatchingMapper
}

// ListAll returns all seeded identity mappers (READ-ONLY, for debugging/admin)
func (r *InMemoryRegistry) ListAll(ctx context.Context) ([]*domain.IdentityMapper, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mappers := make([]*domain.IdentityMapper, 0, len(r.mappers))
	for _, mapper := range r.mappers {
		mappers = append(mappers, mapper)
	}

	return mappers, nil
}

// Ensure InMemoryRegistry implements IdentityMapperRegistry
var _ ports.IdentityMapperRegistry = (*InMemoryRegistry)(nil)
