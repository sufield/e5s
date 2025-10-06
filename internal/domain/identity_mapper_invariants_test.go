package domain_test

import (
	"testing"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIdentityMapper_Invariant_NamespaceNeverNil tests the invariant:
// "identityNamespace is never nil after construction"
func TestIdentityMapper_Invariant_NamespaceNeverNil(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
	selectors := domain.NewSelectorSet()
	sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
	selectors.Add(sel)

	// Act
	mapper, err := domain.NewIdentityMapper(namespace, selectors)

	// Assert invariant: identityNamespace is never nil
	require.NoError(t, err)
	require.NotNil(t, mapper)
	assert.NotNil(t, mapper.IdentityNamespace(),
		"Invariant violated: IdentityNamespace() returned nil")
}

// TestIdentityMapper_Invariant_NamespaceNilRejected tests the invariant:
// "Construction fails if identityNamespace is nil"
func TestIdentityMapper_Invariant_NamespaceNilRejected(t *testing.T) {
	t.Parallel()

	// Arrange
	selectors := domain.NewSelectorSet()
	sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
	selectors.Add(sel)

	// Act
	mapper, err := domain.NewIdentityMapper(nil, selectors)

	// Assert invariant: nil namespace is rejected
	assert.Error(t, err, "Invariant enforced: nil namespace should be rejected")
	assert.ErrorIs(t, err, domain.ErrInvalidIdentityNamespace)
	assert.Nil(t, mapper)
}

// TestIdentityMapper_Invariant_SelectorsNeverNilOrEmpty tests the invariant:
// "selectors is never nil or empty after construction"
func TestIdentityMapper_Invariant_SelectorsNeverNilOrEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupSelectors func() *domain.SelectorSet
		expectError  bool
	}{
		{
			name: "valid selectors",
			setupSelectors: func() *domain.SelectorSet {
				set := domain.NewSelectorSet()
				sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
				set.Add(sel)
				return set
			},
			expectError: false,
		},
		{
			name: "nil selectors violate invariant",
			setupSelectors: func() *domain.SelectorSet {
				return nil
			},
			expectError: true,
		},
		{
			name: "empty selectors violate invariant",
			setupSelectors: func() *domain.SelectorSet {
				return domain.NewSelectorSet() // Empty
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName("example.org")
			namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
			selectors := tt.setupSelectors()

			// Act
			mapper, err := domain.NewIdentityMapper(namespace, selectors)

			// Assert invariant
			if tt.expectError {
				assert.Error(t, err, "Invariant enforced: invalid selectors should be rejected")
				assert.ErrorIs(t, err, domain.ErrInvalidSelectors)
				assert.Nil(t, mapper)
			} else {
				require.NoError(t, err)
				require.NotNil(t, mapper)
				assert.NotNil(t, mapper.Selectors(),
					"Invariant violated: Selectors() returned nil")
				assert.Greater(t, len(mapper.Selectors().All()), 0,
					"Invariant violated: selectors are empty")
			}
		})
	}
}

// TestIdentityMapper_Invariant_MatchesSelectorsANDLogic tests the invariant:
// "MatchesSelectors() uses AND logic (ALL mapper selectors must be present)"
func TestIdentityMapper_Invariant_MatchesSelectorsANDLogic(t *testing.T) {
	t.Parallel()

	// Arrange - Create mapper requiring TWO selectors
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")

	mapperSelectors := domain.NewSelectorSet()
	sel1, _ := domain.ParseSelectorFromString("unix:uid:1000")
	sel2, _ := domain.ParseSelectorFromString("k8s:namespace:prod")
	mapperSelectors.Add(sel1)
	mapperSelectors.Add(sel2)

	mapper, err := domain.NewIdentityMapper(namespace, mapperSelectors)
	require.NoError(t, err)

	tests := []struct {
		name            string
		workloadSelectors []string
		shouldMatch     bool
	}{
		{
			name:            "has both required selectors - matches",
			workloadSelectors: []string{"unix:uid:1000", "k8s:namespace:prod"},
			shouldMatch:     true,
		},
		{
			name:            "has both + extra selectors - matches",
			workloadSelectors: []string{"unix:uid:1000", "k8s:namespace:prod", "k8s:pod:web"},
			shouldMatch:     true,
		},
		{
			name:            "missing one required selector - no match",
			workloadSelectors: []string{"unix:uid:1000"}, // Missing k8s:namespace:prod
			shouldMatch:     false,
		},
		{
			name:            "missing other required selector - no match",
			workloadSelectors: []string{"k8s:namespace:prod"}, // Missing unix:uid:1000
			shouldMatch:     false,
		},
		{
			name:            "has neither required selector - no match",
			workloadSelectors: []string{"unix:gid:2000"},
			shouldMatch:     false,
		},
		{
			name:            "empty selectors - no match",
			workloadSelectors: []string{},
			shouldMatch:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange workload selectors
			workloadSet := domain.NewSelectorSet()
			for _, selStr := range tt.workloadSelectors {
				sel, _ := domain.ParseSelectorFromString(selStr)
				workloadSet.Add(sel)
			}

			// Act
			matches := mapper.MatchesSelectors(workloadSet)

			// Assert invariant: AND logic
			assert.Equal(t, tt.shouldMatch, matches,
				"Invariant violated: MatchesSelectors should use AND logic (all required selectors must be present)")
		})
	}
}

// TestIdentityMapper_Invariant_MatchesSelectorsAllRequired tests the invariant:
// "Returns true iff ALL selectors in im.selectors are contained in input selectors"
func TestIdentityMapper_Invariant_MatchesSelectorsAllRequired(t *testing.T) {
	t.Parallel()

	// Arrange - Create mapper with 3 required selectors
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")

	mapperSelectors := domain.NewSelectorSet()
	sel1, _ := domain.ParseSelectorFromString("unix:uid:1000")
	sel2, _ := domain.ParseSelectorFromString("k8s:namespace:prod")
	sel3, _ := domain.ParseSelectorFromString("k8s:pod:web")
	mapperSelectors.Add(sel1)
	mapperSelectors.Add(sel2)
	mapperSelectors.Add(sel3)

	mapper, err := domain.NewIdentityMapper(namespace, mapperSelectors)
	require.NoError(t, err)

	// Test case: workload has all 3
	workloadAll := domain.NewSelectorSet()
	workloadAll.Add(sel1)
	workloadAll.Add(sel2)
	workloadAll.Add(sel3)
	assert.True(t, mapper.MatchesSelectors(workloadAll),
		"Invariant violated: should match when ALL required selectors present")

	// Test case: workload missing 1
	workloadMissing := domain.NewSelectorSet()
	workloadMissing.Add(sel1)
	workloadMissing.Add(sel2)
	// Missing sel3
	assert.False(t, mapper.MatchesSelectors(workloadMissing),
		"Invariant violated: should NOT match when ANY required selector missing")
}

// TestIdentityMapper_Invariant_MatchesSelectorsConsistency tests the invariant:
// "Multiple calls with same input return same result (deterministic)"
func TestIdentityMapper_Invariant_MatchesSelectorsConsistency(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")

	mapperSelectors := domain.NewSelectorSet()
	sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
	mapperSelectors.Add(sel)

	mapper, err := domain.NewIdentityMapper(namespace, mapperSelectors)
	require.NoError(t, err)

	workloadSet := domain.NewSelectorSet()
	workloadSet.Add(sel)

	// Act - Call multiple times
	result1 := mapper.MatchesSelectors(workloadSet)
	result2 := mapper.MatchesSelectors(workloadSet)
	result3 := mapper.MatchesSelectors(workloadSet)

	// Assert invariant: consistent results
	assert.Equal(t, result1, result2, "Invariant violated: MatchesSelectors should be deterministic")
	assert.Equal(t, result2, result3, "Invariant violated: MatchesSelectors should be deterministic")
	assert.True(t, result1, "Should match")
}
