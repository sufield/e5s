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
	mappers map[string]*domain.IdentityMapper // identity format string -> mapper
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

	identityFormatStr := mapper.IdentityNamespace().String()
	if _, exists := r.mappers[identityFormatStr]; exists {
		return fmt.Errorf("identity mapper already exists for identity format: %s", identityFormatStr)
	}

	r.mappers[identityFormatStr] = mapper
	return nil
}

// FindMatchingMapper finds an identity mapper that matches the given selectors
// This is the core authorization logic: given workload selectors, which identity format should be issued?
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

// DeleteMapper deletes an identity mapper by identity format
func (r *InMemoryIdentityMapperRepository) DeleteMapper(ctx context.Context, identityFormat *domain.IdentityNamespace) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if identityFormat == nil {
		return fmt.Errorf("identity format cannot be nil")
	}

	identityFormatStr := identityFormat.String()
	if _, exists := r.mappers[identityFormatStr]; !exists {
		return fmt.Errorf("identity mapper not found for identity format: %s", identityFormatStr)
	}

	delete(r.mappers, identityFormatStr)
	return nil
}

// Ensure InMemoryIdentityMapperRepository implements the port
var _ ports.IdentityMapperRepository = (*InMemoryIdentityMapperRepository)(nil)
