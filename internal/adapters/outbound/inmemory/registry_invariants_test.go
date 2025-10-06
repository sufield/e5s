package inmemory_test

import (
	"context"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistry_Invariant_ImmutableAfterSealing tests the invariant:
// "Registry is immutable after sealing"
func TestRegistry_Invariant_ImmutableAfterSealing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := inmemory.NewInMemoryRegistry()

	// Arrange - Seed and seal
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
	selectors := domain.NewSelectorSet()
	sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
	selectors.Add(sel)

	mapper, err := domain.NewIdentityMapper(namespace, selectors)
	require.NoError(t, err)

	err = registry.Seed(ctx, mapper)
	require.NoError(t, err)

	// Act - Seal the registry
	registry.Seal()

	// Try to seed after sealing
	namespace2 := domain.NewIdentityNamespaceFromComponents(td, "/service")
	mapper2, err := domain.NewIdentityMapper(namespace2, selectors)
	require.NoError(t, err)

	err = registry.Seed(ctx, mapper2)

	// Assert invariant: Seed fails after sealing
	assert.Error(t, err, "Invariant violated: Seed should fail after sealing")
	assert.ErrorIs(t, err, domain.ErrRegistrySealed,
		"Invariant violated: should return ErrRegistrySealed")

	// Verify original mapper still exists and new one wasn't added
	mappers, err := registry.ListAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, len(mappers), "Invariant violated: sealed registry was modified")
}

// TestRegistry_Invariant_SeedRejectsDuplicates tests the invariant:
// "Seed() rejects duplicates by identity namespace"
func TestRegistry_Invariant_SeedRejectsDuplicates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := inmemory.NewInMemoryRegistry()

	// Arrange - Create two mappers with same namespace
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")

	selectors1 := domain.NewSelectorSet()
	sel1, _ := domain.ParseSelectorFromString("unix:uid:1000")
	selectors1.Add(sel1)

	selectors2 := domain.NewSelectorSet()
	sel2, _ := domain.ParseSelectorFromString("unix:uid:2000")
	selectors2.Add(sel2)

	mapper1, err := domain.NewIdentityMapper(namespace, selectors1)
	require.NoError(t, err)

	mapper2, err := domain.NewIdentityMapper(namespace, selectors2) // Same namespace!
	require.NoError(t, err)

	// Act - Seed first mapper
	err = registry.Seed(ctx, mapper1)
	require.NoError(t, err)

	// Try to seed duplicate namespace
	err = registry.Seed(ctx, mapper2)

	// Assert invariant: duplicate rejected
	assert.Error(t, err, "Invariant violated: should reject duplicate namespace")
	assert.Contains(t, err.Error(), "already exists")

	// Verify only first mapper exists
	mappers, err := registry.ListAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, len(mappers), "Invariant violated: duplicate was added")
	assert.Equal(t, mapper1.IdentityNamespace(), mappers[0].IdentityNamespace())
}

// TestRegistry_Invariant_FindBySelectorsReadOnly tests the invariant:
// "FindBySelectors() is read-only (never modifies registry)"
func TestRegistry_Invariant_FindBySelectorsReadOnly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := inmemory.NewInMemoryRegistry()

	// Arrange - Seed registry
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
	selectors := domain.NewSelectorSet()
	sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
	selectors.Add(sel)

	mapper, err := domain.NewIdentityMapper(namespace, selectors)
	require.NoError(t, err)

	err = registry.Seed(ctx, mapper)
	require.NoError(t, err)

	registry.Seal()

	// Get initial state
	mappersBefore, err := registry.ListAll(ctx)
	require.NoError(t, err)
	countBefore := len(mappersBefore)

	// Act - Call FindBySelectors multiple times
	for i := 0; i < 10; i++ {
		_, _ = registry.FindBySelectors(ctx, selectors) // Ignore result
	}

	// Assert invariant: registry unchanged
	mappersAfter, err := registry.ListAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, countBefore, len(mappersAfter),
		"Invariant violated: FindBySelectors modified the registry")
}

// TestRegistry_Invariant_FindBySelectorsValidatesInput tests the invariant:
// "FindBySelectors() validates input before search"
func TestRegistry_Invariant_FindBySelectorsValidatesInput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := inmemory.NewInMemoryRegistry()

	tests := []struct {
		name        string
		selectors   *domain.SelectorSet
		expectError bool
	}{
		{
			name:        "nil selectors - invalid",
			selectors:   nil,
			expectError: true,
		},
		{
			name:        "empty selectors - invalid",
			selectors:   domain.NewSelectorSet(),
			expectError: true,
		},
		{
			name: "valid selectors",
			selectors: func() *domain.SelectorSet {
				set := domain.NewSelectorSet()
				sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
				set.Add(sel)
				return set
			}(),
			expectError: false, // No error, but might not find (ErrNoMatchingMapper)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := registry.FindBySelectors(ctx, tt.selectors)

			// Assert invariant: validates input
			if tt.expectError {
				assert.Error(t, err, "Invariant violated: should validate input")
				assert.ErrorIs(t, err, domain.ErrInvalidSelectors)
			}
			// Note: even valid selectors might return ErrNoMatchingMapper if registry empty
		})
	}
}

// TestRegistry_Invariant_FindBySelectorsANDLogic tests the invariant:
// "FindBySelectors() returns first match using AND logic"
func TestRegistry_Invariant_FindBySelectorsANDLogic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := inmemory.NewInMemoryRegistry()

	// Arrange - Create mapper requiring 2 selectors
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")

	mapperSelectors := domain.NewSelectorSet()
	sel1, _ := domain.ParseSelectorFromString("unix:uid:1000")
	sel2, _ := domain.ParseSelectorFromString("k8s:namespace:prod")
	mapperSelectors.Add(sel1)
	mapperSelectors.Add(sel2)

	mapper, err := domain.NewIdentityMapper(namespace, mapperSelectors)
	require.NoError(t, err)

	err = registry.Seed(ctx, mapper)
	require.NoError(t, err)
	registry.Seal()

	tests := []struct {
		name        string
		selectors   []string
		shouldMatch bool
	}{
		{
			name:        "has both required selectors - matches",
			selectors:   []string{"unix:uid:1000", "k8s:namespace:prod"},
			shouldMatch: true,
		},
		{
			name:        "has both + extra - matches",
			selectors:   []string{"unix:uid:1000", "k8s:namespace:prod", "k8s:pod:web"},
			shouldMatch: true,
		},
		{
			name:        "missing one required - no match",
			selectors:   []string{"unix:uid:1000"},
			shouldMatch: false,
		},
		{
			name:        "has neither - no match",
			selectors:   []string{"unix:gid:2000"},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange workload selectors
			workloadSet := domain.NewSelectorSet()
			for _, selStr := range tt.selectors {
				sel, _ := domain.ParseSelectorFromString(selStr)
				workloadSet.Add(sel)
			}

			// Act
			found, err := registry.FindBySelectors(ctx, workloadSet)

			// Assert invariant: AND logic
			if tt.shouldMatch {
				require.NoError(t, err, "Should find match")
				assert.NotNil(t, found)
				assert.Equal(t, namespace, found.IdentityNamespace())
			} else {
				assert.Error(t, err, "Should not find match")
				assert.ErrorIs(t, err, domain.ErrNoMatchingMapper)
				assert.Nil(t, found)
			}
		})
	}
}

// TestRegistry_Invariant_ListAllNeverReturnsNilSlice tests the invariant:
// "ListAll() never returns nil slice when mappers exist"
func TestRegistry_Invariant_ListAllNeverReturnsNilSlice(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name          string
		setupRegistry func() *inmemory.InMemoryRegistry
		expectError   bool
		expectNonNil  bool
	}{
		{
			name: "registry with mappers - returns non-nil slice",
			setupRegistry: func() *inmemory.InMemoryRegistry {
				reg := inmemory.NewInMemoryRegistry()
				td := domain.NewTrustDomainFromName("example.org")
				namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
				selectors := domain.NewSelectorSet()
				sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
				selectors.Add(sel)
				mapper, _ := domain.NewIdentityMapper(namespace, selectors)
				_ = reg.Seed(ctx, mapper)
				reg.Seal()
				return reg
			},
			expectError:  false,
			expectNonNil: true,
		},
		{
			name: "empty registry - returns error",
			setupRegistry: func() *inmemory.InMemoryRegistry {
				reg := inmemory.NewInMemoryRegistry()
				reg.Seal()
				return reg
			},
			expectError:  true,
			expectNonNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			registry := tt.setupRegistry()

			// Act
			mappers, err := registry.ListAll(ctx)

			// Assert invariant
			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, domain.ErrRegistryEmpty)
			} else {
				require.NoError(t, err)
				if tt.expectNonNil {
					assert.NotNil(t, mappers,
						"Invariant violated: ListAll should not return nil slice when mappers exist")
					assert.Greater(t, len(mappers), 0)
				}
			}
		})
	}
}

// TestRegistry_Invariant_SealIsOneWay tests the invariant:
// "Registry transitions: Unsealed (mutable) → Sealed (immutable), never reversed"
func TestRegistry_Invariant_SealIsOneWay(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := inmemory.NewInMemoryRegistry()

	// Arrange - Seed before sealing
	td := domain.NewTrustDomainFromName("example.org")
	namespace1 := domain.NewIdentityNamespaceFromComponents(td, "/workload1")
	selectors := domain.NewSelectorSet()
	sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
	selectors.Add(sel)

	mapper1, err := domain.NewIdentityMapper(namespace1, selectors)
	require.NoError(t, err)

	// Act - Seed before seal (should work)
	err = registry.Seed(ctx, mapper1)
	assert.NoError(t, err, "Seeding before seal should work")

	// Seal the registry
	registry.Seal()

	// Try to seed after sealing (should fail)
	namespace2 := domain.NewIdentityNamespaceFromComponents(td, "/workload2")
	mapper2, err := domain.NewIdentityMapper(namespace2, selectors)
	require.NoError(t, err)

	err = registry.Seed(ctx, mapper2)
	assert.Error(t, err, "Invariant violated: Seed should fail after seal")

	// Seal again (should be idempotent)
	registry.Seal()

	// Try to seed after second seal (should still fail)
	namespace3 := domain.NewIdentityNamespaceFromComponents(td, "/workload3")
	mapper3, err := domain.NewIdentityMapper(namespace3, selectors)
	require.NoError(t, err)

	err = registry.Seed(ctx, mapper3)
	assert.Error(t, err, "Invariant violated: Seal is permanent, cannot be reversed")

	// Verify only first mapper exists
	mappers, err := registry.ListAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, len(mappers), "Invariant violated: sealed registry was modified")
}

// TestRegistry_Invariant_ListAllWorksAfterSealing tests the invariant:
// "ListAll() works on sealed registry (read-only operation)"
func TestRegistry_Invariant_ListAllWorksAfterSealing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := inmemory.NewInMemoryRegistry()

	// Arrange - Seed multiple mappers
	td := domain.NewTrustDomainFromName("example.org")
	for i := 0; i < 5; i++ {
		namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload"+string(rune('0'+i)))
		selectors := domain.NewSelectorSet()
		sel, _ := domain.ParseSelectorFromString("unix:uid:100" + string(rune('0'+i)))
		selectors.Add(sel)
		mapper, _ := domain.NewIdentityMapper(namespace, selectors)
		_ = registry.Seed(ctx, mapper)
	}

	// Seal the registry
	registry.Seal()

	// Act - ListAll should still work (read-only)
	mappers, err := registry.ListAll(ctx)

	// Assert invariant: ListAll works on sealed registry
	require.NoError(t, err, "Invariant violated: ListAll should work on sealed registry")
	assert.Equal(t, 5, len(mappers), "Should return all seeded mappers")
}
