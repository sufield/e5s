package inmemory_test

// InMemory Registry Invariant Tests
//
// These tests verify domain invariants for the InMemory identity mapper registry.
// Invariants tested: immutability after sealing, duplicate rejection, read-only operations,
// AND selector logic, and state transitions (unsealed → sealed).
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/inmemory/... -v -run TestRegistry_Invariant
//	go test ./internal/adapters/outbound/inmemory/... -cover

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
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
	selectors := domain.NewSelectorSet()
	sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
	selectors.Add(sel)

	mapper, err := domain.NewIdentityMapper(credential, selectors)
	require.NoError(t, err)

	err = registry.Seed(ctx, mapper)
	require.NoError(t, err)

	// Act - Seal the registry
	registry.Seal()

	// Try to seed after sealing
	credential2 := domain.NewIdentityCredentialFromComponents(td, "/service")
	mapper2, err := domain.NewIdentityMapper(credential2, selectors)
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
// "Seed() rejects duplicates by identity credential"
func TestRegistry_Invariant_SeedRejectsDuplicates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := inmemory.NewInMemoryRegistry()

	// Arrange - Create two mappers with same credential
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")

	selectors1 := domain.NewSelectorSet()
	sel1, _ := domain.ParseSelectorFromString("unix:uid:1000")
	selectors1.Add(sel1)

	selectors2 := domain.NewSelectorSet()
	sel2, _ := domain.ParseSelectorFromString("unix:uid:2000")
	selectors2.Add(sel2)

	mapper1, err := domain.NewIdentityMapper(credential, selectors1)
	require.NoError(t, err)

	mapper2, err := domain.NewIdentityMapper(credential, selectors2) // Same credential!
	require.NoError(t, err)

	// Act - Seed first mapper
	err = registry.Seed(ctx, mapper1)
	require.NoError(t, err)

	// Try to seed duplicate credential
	err = registry.Seed(ctx, mapper2)

	// Assert invariant: duplicate rejected
	assert.Error(t, err, "Invariant violated: should reject duplicate credential")
	assert.Contains(t, err.Error(), "already exists")

	// Verify only first mapper exists
	mappers, err := registry.ListAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, len(mappers), "Invariant violated: duplicate was added")
	assert.Equal(t, mapper1.IdentityCredential(), mappers[0].IdentityCredential())
}

// TestRegistry_Invariant_FindBySelectorsReadOnly tests the invariant:
// "FindBySelectors() is read-only (never modifies registry)"
func TestRegistry_Invariant_FindBySelectorsReadOnly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := inmemory.NewInMemoryRegistry()

	// Arrange - Seed registry
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
	selectors := domain.NewSelectorSet()
	sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
	selectors.Add(sel)

	mapper, err := domain.NewIdentityMapper(credential, selectors)
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
		name      string
		selectors *domain.SelectorSet
		wantError bool
	}{
		{
			name:      "nil selectors - invalid",
			selectors: nil,
			wantError: true,
		},
		{
			name:      "empty selectors - invalid",
			selectors: domain.NewSelectorSet(),
			wantError: true,
		},
		{
			name: "valid selectors",
			selectors: func() *domain.SelectorSet {
				set := domain.NewSelectorSet()
				sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
				set.Add(sel)
				return set
			}(),
			wantError: false, // No error, but might not find (ErrNoMatchingMapper)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := registry.FindBySelectors(ctx, tt.selectors)

			// Assert invariant: validates input
			if tt.wantError {
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
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")

	mapperSelectors := domain.NewSelectorSet()
	sel1, _ := domain.ParseSelectorFromString("unix:uid:1000")
	sel2, _ := domain.ParseSelectorFromString("k8s:namespace:prod")
	mapperSelectors.Add(sel1)
	mapperSelectors.Add(sel2)

	mapper, err := domain.NewIdentityMapper(credential, mapperSelectors)
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
				assert.Equal(t, credential, found.IdentityCredential())
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
		wantError     bool
		expectNonNil  bool
	}{
		{
			name: "registry with mappers - returns non-nil slice",
			setupRegistry: func() *inmemory.InMemoryRegistry {
				reg := inmemory.NewInMemoryRegistry()
				td := domain.NewTrustDomainFromName("example.org")
				credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
				selectors := domain.NewSelectorSet()
				sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
				selectors.Add(sel)
				mapper, _ := domain.NewIdentityMapper(credential, selectors)
				_ = reg.Seed(ctx, mapper)
				reg.Seal()
				return reg
			},
			wantError:    false,
			expectNonNil: true,
		},
		{
			name: "empty registry - returns error",
			setupRegistry: func() *inmemory.InMemoryRegistry {
				reg := inmemory.NewInMemoryRegistry()
				reg.Seal()
				return reg
			},
			wantError:    true,
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
			if tt.wantError {
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
	namespace1 := domain.NewIdentityCredentialFromComponents(td, "/workload1")
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
	namespace2 := domain.NewIdentityCredentialFromComponents(td, "/workload2")
	mapper2, err := domain.NewIdentityMapper(namespace2, selectors)
	require.NoError(t, err)

	err = registry.Seed(ctx, mapper2)
	assert.Error(t, err, "Invariant violated: Seed should fail after seal")

	// Seal again (should be idempotent)
	registry.Seal()

	// Try to seed after second seal (should still fail)
	namespace3 := domain.NewIdentityCredentialFromComponents(td, "/workload3")
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
		credential := domain.NewIdentityCredentialFromComponents(td, "/workload"+string(rune('0'+i)))
		selectors := domain.NewSelectorSet()
		sel, _ := domain.ParseSelectorFromString("unix:uid:100" + string(rune('0'+i)))
		selectors.Add(sel)
		mapper, _ := domain.NewIdentityMapper(credential, selectors)
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
