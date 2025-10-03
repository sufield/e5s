package inmemory

import (
	"context"
	"fmt"
	"sync"

	"github.com/pocket/hexagon/spire/internal/app/ports"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// InMemoryStore is an in-memory implementation of IdentityStore
type InMemoryStore struct {
	mu         sync.RWMutex
	identities map[string]*registeredIdentity // identity format string -> identity
	selectors  map[string]string               // selector string -> identity format string
}

type registeredIdentity struct {
	identityFormat *domain.IdentityNamespace
	selector       *domain.Selector
}

// NewInMemoryStore creates a new in-memory identity store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		identities: make(map[string]*registeredIdentity),
		selectors:  make(map[string]string),
	}
}

// Register registers a new workload identity with a selector
func (s *InMemoryStore) Register(ctx context.Context, identityFormat *domain.IdentityNamespace, selector *domain.Selector) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idStr := identityFormat.String()
	if _, exists := s.identities[idStr]; exists {
		return fmt.Errorf("identity %s already registered", idStr)
	}

	s.identities[idStr] = &registeredIdentity{
		identityFormat: identityFormat,
		selector:       selector,
	}
	s.selectors[selector.String()] = idStr

	return nil
}

// GetIdentity retrieves an identity by identity format
func (s *InMemoryStore) GetIdentity(ctx context.Context, identityFormat *domain.IdentityNamespace) (*ports.Identity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idStr := identityFormat.String()
	reg, exists := s.identities[idStr]
	if !exists {
		return nil, fmt.Errorf("identity %s not found", idStr)
	}

	return domainToIdentity(reg.identityFormat, nil), nil
}

// GetIdentityBySelector retrieves an identity by selector (internal helper)
func (s *InMemoryStore) GetIdentityBySelector(ctx context.Context, selectorStr string) (*ports.Identity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	identityFormatStr, exists := s.selectors[selectorStr]
	if !exists {
		return nil, fmt.Errorf("no identity found for selector %s", selectorStr)
	}

	reg, exists := s.identities[identityFormatStr]
	if !exists {
		return nil, fmt.Errorf("identity %s not found", identityFormatStr)
	}

	return domainToIdentity(reg.identityFormat, nil), nil
}

// ListIdentities lists all registered identities
func (s *InMemoryStore) ListIdentities(ctx context.Context) ([]*ports.Identity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	identities := make([]*ports.Identity, 0, len(s.identities))
	for _, reg := range s.identities {
		identities = append(identities, domainToIdentity(reg.identityFormat, nil))
	}

	return identities, nil
}
