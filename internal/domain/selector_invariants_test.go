//go:build dev

package domain_test

// Selector Invariant Tests
//
// These tests verify domain invariants for Selector and SelectorSet value objects.
// Invariants tested: key/value non-empty, format consistency, parsing rules,
// equality properties (reflexive, symmetric, transitive), set uniqueness.
//
// Run these tests with:
//
//	go test ./internal/domain/... -v -run TestSelector_Invariant
//	go test ./internal/domain/... -v -run TestSelectorSet_Invariant
//	go test ./internal/domain/... -cover

import (
	"testing"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSelector_Invariant_KeyValueNeverEmpty tests the invariant:
// "key and value are never empty after construction"
func TestSelector_Invariant_KeyValueNeverEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		selectorType domain.SelectorType
		key          string
		value        string
		wantError    bool
	}{
		{
			name:         "valid selector",
			selectorType: "unix",
			key:          "uid",
			value:        "1000",
			wantError:    false,
		},
		{
			name:         "empty key violates invariant",
			selectorType: "unix",
			key:          "",
			value:        "1000",
			wantError:    true,
		},
		{
			name:         "empty value violates invariant",
			selectorType: "unix",
			key:          "uid",
			value:        "",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			sel, err := domain.NewSelector(tt.selectorType, tt.key, tt.value)

			// Assert invariant: key and value never empty if no error
			if tt.wantError {
				assert.Error(t, err, "Expected error for invalid input")
				assert.Nil(t, sel)
			} else {
				require.NoError(t, err)
				require.NotNil(t, sel)
				assert.NotEmpty(t, sel.Key(), "Invariant violated: key is empty")
				assert.NotEmpty(t, sel.Value(), "Invariant violated: value is empty")
			}
		})
	}
}

// TestSelector_Invariant_FormattedMatchesComponents tests the invariant:
// "formatted matches 'type:key:value' pattern"
func TestSelector_Invariant_FormattedMatchesComponents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		selectorType domain.SelectorType
		key          string
		value        string
		expectedStr  string
	}{
		{
			name:         "unix uid",
			selectorType: "unix",
			key:          "uid",
			value:        "1000",
			expectedStr:  "unix:uid:1000",
		},
		{
			name:         "k8s namespace",
			selectorType: "k8s",
			key:          "namespace",
			value:        "prod",
			expectedStr:  "k8s:namespace:prod",
		},
		{
			name:         "value with colon",
			selectorType: "custom",
			key:          "key",
			value:        "value:with:colons",
			expectedStr:  "custom:key:value:with:colons",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			sel, err := domain.NewSelector(tt.selectorType, tt.key, tt.value)

			// Assert invariant: formatted matches components
			require.NoError(t, err)
			require.NotNil(t, sel)
			assert.Equal(t, tt.expectedStr, sel.String(),
				"Invariant violated: formatted string must match 'type:key:value'")
		})
	}
}

// TestSelector_Invariant_ParseRequiresThreeParts tests the invariant:
// "ParseSelectorFromString requires at least 3 parts (type:key:value)"
func TestSelector_Invariant_ParseRequiresThreeParts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantError     bool
		expectedType  string
		expectedKey   string
		expectedValue string
	}{
		{
			name:          "valid 3 parts",
			input:         "unix:uid:1000",
			wantError:     false,
			expectedType:  "unix",
			expectedKey:   "uid",
			expectedValue: "1000",
		},
		{
			name:          "valid with colon in value",
			input:         "custom:key:value:with:colons",
			wantError:     false,
			expectedType:  "custom",
			expectedKey:   "key",
			expectedValue: "value:with:colons",
		},
		{
			name:      "invalid 2 parts",
			input:     "unix:uid",
			wantError: true,
		},
		{
			name:      "invalid 1 part",
			input:     "unix",
			wantError: true,
		},
		{
			name:      "invalid empty",
			input:     "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			sel, err := domain.ParseSelectorFromString(tt.input)

			// Assert invariant: requires at least 3 parts
			if tt.wantError {
				assert.Error(t, err, "Expected error for invalid format")
				assert.Nil(t, sel)
			} else {
				require.NoError(t, err)
				require.NotNil(t, sel)
				assert.Equal(t, domain.SelectorType(tt.expectedType), sel.Type())
				assert.Equal(t, tt.expectedKey, sel.Key())
				assert.Equal(t, tt.expectedValue, sel.Value())
				assert.NotEmpty(t, sel.Type(), "Invariant violated: type is empty")
				assert.NotEmpty(t, sel.Key(), "Invariant violated: key is empty")
				assert.NotEmpty(t, sel.Value(), "Invariant violated: value is empty")
			}
		})
	}
}

// TestSelector_Invariant_EqualsReflexive tests the invariant:
// "Equals() is reflexive"
func TestSelector_Invariant_EqualsReflexive(t *testing.T) {
	t.Parallel()

	// Arrange
	sel, err := domain.ParseSelectorFromString("unix:uid:1000")
	require.NoError(t, err)

	// Assert invariant: reflexive
	assert.True(t, sel.Equals(sel), "Invariant violated: Equals() is not reflexive")
}

// TestSelector_Invariant_EqualsSymmetric tests the invariant:
// "Equals() is symmetric"
func TestSelector_Invariant_EqualsSymmetric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		sel1   string
		sel2   string
		equals bool
	}{
		{"same selector", "unix:uid:1000", "unix:uid:1000", true},
		{"different values", "unix:uid:1000", "unix:uid:1001", false},
		{"different types", "unix:uid:1000", "k8s:uid:1000", false},
		{"different keys", "unix:uid:1000", "unix:gid:1000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			s1, err1 := domain.ParseSelectorFromString(tt.sel1)
			s2, err2 := domain.ParseSelectorFromString(tt.sel2)
			require.NoError(t, err1)
			require.NoError(t, err2)

			// Assert invariant: symmetric
			assert.Equal(t, s1.Equals(s2), s2.Equals(s1),
				"Invariant violated: Equals() is not symmetric")
			assert.Equal(t, tt.equals, s1.Equals(s2))
		})
	}
}

// TestSelector_Invariant_EqualsTransitive tests the invariant:
// "Equals() is transitive"
func TestSelector_Invariant_EqualsTransitive(t *testing.T) {
	t.Parallel()

	// Arrange
	s1, _ := domain.ParseSelectorFromString("unix:uid:1000")
	s2, _ := domain.ParseSelectorFromString("unix:uid:1000")
	s3, _ := domain.ParseSelectorFromString("unix:uid:1000")

	// Assert invariant: transitive
	assert.True(t, s1.Equals(s2), "Setup: s1 should equal s2")
	assert.True(t, s2.Equals(s3), "Setup: s2 should equal s3")
	assert.True(t, s1.Equals(s3), "Invariant violated: Equals() is not transitive")
}

// TestSelector_Invariant_EqualsNilSafe tests the invariant:
// "Equals() returns false for nil input, never panics"
func TestSelector_Invariant_EqualsNilSafe(t *testing.T) {
	t.Parallel()

	// Arrange
	sel, err := domain.ParseSelectorFromString("unix:uid:1000")
	require.NoError(t, err)

	// Assert invariant: nil-safe
	assert.NotPanics(t, func() {
		result := sel.Equals(nil)
		assert.False(t, result, "Invariant violated: Equals(nil) should return false")
	})
}

// TestSelectorSet_Invariant_NoD duplicates tests the invariant:
// "Set contains no duplicate selectors (uniqueness)"
func TestSelectorSet_Invariant_NoDuplicates(t *testing.T) {
	t.Parallel()

	// Arrange
	set := domain.NewSelectorSet()
	sel1, _ := domain.ParseSelectorFromString("unix:uid:1000")
	sel2, _ := domain.ParseSelectorFromString("unix:uid:1000") // Duplicate

	// Act
	initialSize := len(set.All())
	set.Add(sel1)
	sizeAfterFirst := len(set.All())
	set.Add(sel2) // Add duplicate
	sizeAfterSecond := len(set.All())

	// Assert invariant: no duplicates
	assert.Equal(t, 0, initialSize, "Set should start empty")
	assert.Equal(t, 1, sizeAfterFirst, "Set should have 1 element after first add")
	assert.Equal(t, 1, sizeAfterSecond, "Invariant violated: duplicate was added to set")
	assert.True(t, set.Contains(sel1), "Set should contain the selector")
	assert.True(t, set.Contains(sel2), "Set should contain duplicate selector (same value)")
}

// TestSelectorSet_Invariant_AddEnsuresContains tests the invariant:
// "After Add(s), ss.Contains(s) == true"
func TestSelectorSet_Invariant_AddEnsuresContains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		selector string
	}{
		{"unix uid", "unix:uid:1000"},
		{"k8s namespace", "k8s:namespace:prod"},
		{"complex value", "custom:key:value:with:colons"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			set := domain.NewSelectorSet()
			sel, _ := domain.ParseSelectorFromString(tt.selector)

			// Act
			set.Add(sel)

			// Assert invariant: Contains returns true after Add
			assert.True(t, set.Contains(sel),
				"Invariant violated: Contains(s) should be true after Add(s)")
		})
	}
}

// TestSelectorSet_Invariant_ContainsNeverModifies tests the invariant:
// "Contains() never modifies the set"
func TestSelectorSet_Invariant_ContainsNeverModifies(t *testing.T) {
	t.Parallel()

	// Arrange
	set := domain.NewSelectorSet()
	sel1, _ := domain.ParseSelectorFromString("unix:uid:1000")
	sel2, _ := domain.ParseSelectorFromString("k8s:namespace:prod")
	set.Add(sel1)

	// Act
	sizeBefore := len(set.All())
	_ = set.Contains(sel1) // Should not modify
	_ = set.Contains(sel2) // Should not modify
	sizeAfter := len(set.All())

	// Assert invariant: Contains doesn't modify set
	assert.Equal(t, sizeBefore, sizeAfter,
		"Invariant violated: Contains() modified the set")
	assert.Equal(t, 1, sizeAfter)
}

// TestSelectorSet_Invariant_AllReturnsDefensiveCopy tests the invariant:
// "All() returns defensive copy to prevent external mutation"
func TestSelectorSet_Invariant_AllReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()

	// Arrange
	set := domain.NewSelectorSet()
	sel1, _ := domain.ParseSelectorFromString("unix:uid:1000")
	sel2, _ := domain.ParseSelectorFromString("k8s:namespace:prod")
	set.Add(sel1)

	// Act
	copy1 := set.All()
	originalSize := len(copy1)

	// Try to mutate the returned slice
	copy1 = append(copy1, sel2)

	// Get a fresh copy
	copy2 := set.All()

	// Assert invariant: external mutation doesn't affect set
	assert.Equal(t, originalSize, len(copy2),
		"Invariant violated: external mutation affected the set")
	assert.NotEqual(t, len(copy1), len(copy2),
		"Invariant violated: All() did not return a defensive copy")
	assert.False(t, set.Contains(sel2),
		"Invariant violated: set should not contain sel2")
}
