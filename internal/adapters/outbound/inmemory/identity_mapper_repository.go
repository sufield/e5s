package inmemory

import (
	"context"
	"fmt"
	"sync"

	"github.com/pocket/hexagon/spire/internal/app/ports"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// InMemoryIdentityMapperRepository is an in-memory implementation of IdentityMapperRepository
// In real SPIRE, this would interact with the server's datastore (SQL, etc.)
// For walking skeleton, uses simple map-based storage
type InMemoryIdentityMapperRepository struct {
	mu      sync.RWMutex
	mappers map[string]*domain.IdentityMapper // identity namespace string -> mapper
}

// NewInMemoryIdentityMapperRepository creates a new in-memory identity mapper repository
func NewInMemoryIdentityMapperRepository() *InMemoryIdentityMapperRepository {
	return &InMemoryIdentityMapperRepository{
		mappers: make(map[string]*domain.IdentityMapper),
	}
}

// CreateMapper creates a new identity mapper
func (r *InMemoryIdentityMapperRepository) CreateMapper(ctx context.Context, mapper *domain.IdentityMapper) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if mapper == nil {
		return fmt.Errorf("identity mapper cannot be nil")
	}

	identityNamespaceStr := mapper.IdentityNamespace().String()
	if _, exists := r.mappers[identityNamespaceStr]; exists {
		return fmt.Errorf("identity mapper already exists for identity namespace: %s", identityNamespaceStr)
	}

	r.mappers[identityNamespaceStr] = mapper
	return nil
}

// FindMatchingMapper finds an identity mapper that matches the given selectors
// This is the core authorization logic: given workload selectors, which identity namespace should be issued?
func (r *InMemoryIdentityMapperRepository) FindMatchingMapper(ctx context.Context, selectors *domain.SelectorSet) (*domain.IdentityMapper, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if selectors == nil || len(selectors.All()) == 0 {
		return nil, fmt.Errorf("selectors cannot be empty")
	}

	// Find first matching mapper
	// In real SPIRE, this has more complex logic (parent-child relationships, selector subset matching, etc.)
	// For walking skeleton, simple matching logic
	for _, mapper := range r.mappers {
		if mapper.MatchesSelectors(selectors) {
			return mapper, nil
		}
	}

	return nil, fmt.Errorf("no identity mapper found matching selectors")
}

// ListMappers lists all identity mappers
func (r *InMemoryIdentityMapperRepository) ListMappers(ctx context.Context) ([]*domain.IdentityMapper, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mappers := make([]*domain.IdentityMapper, 0, len(r.mappers))
	for _, mapper := range r.mappers {
		mappers = append(mappers, mapper)
	}

	return mappers, nil
}

// DeleteMapper deletes an identity mapper by identity namespace
func (r *InMemoryIdentityMapperRepository) DeleteMapper(ctx context.Context, identityNamespace *domain.IdentityNamespace) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if identityNamespace == nil {
		return fmt.Errorf("identity namespace cannot be nil")
	}

	identityNamespaceStr := identityNamespace.String()
	if _, exists := r.mappers[identityNamespaceStr]; !exists {
		return fmt.Errorf("identity mapper not found for identity namespace: %s", identityNamespaceStr)
	}

	delete(r.mappers, identityNamespaceStr)
	return nil
}

// Ensure InMemoryIdentityMapperRepository implements the port
var _ ports.IdentityMapperRepository = (*InMemoryIdentityMapperRepository)(nil)
