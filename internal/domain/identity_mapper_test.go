package domain_test

import (
	"testing"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIdentityMapper_IsZero tests the IsZero method for detecting uninitialized mappers
func TestIdentityMapper_IsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func() *domain.IdentityMapper
		wantZero bool
	}{
		{
			name: "nil mapper is zero",
			setup: func() *domain.IdentityMapper {
				return nil
			},
			wantZero: true,
		},
		{
			name: "valid mapper is not zero",
			setup: func() *domain.IdentityMapper {
				td := domain.NewTrustDomainFromName("example.org")
				id := domain.NewIdentityCredentialFromComponents(td, "/workload")
				selectors := domain.NewSelectorSet()
				sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
				selectors.Add(sel)
				mapper, _ := domain.NewIdentityMapper(id, selectors)
				return mapper
			},
			wantZero: false,
		},
		{
			name: "mapper with nil credential is zero",
			setup: func() *domain.IdentityMapper {
				// Can't create this through constructor, simulate programming error
				return &domain.IdentityMapper{}
			},
			wantZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			mapper := tt.setup()
			result := mapper.IsZero()

			// Assert
			assert.Equal(t, tt.wantZero, result)
		})
	}
}

// TestIdentityMapper_MatchesSelectors_NilSafety tests nil safety of MatchesSelectors
func TestIdentityMapper_MatchesSelectors_NilSafety(t *testing.T) {
	t.Parallel()

	td := domain.NewTrustDomainFromName("example.org")
	id := domain.NewIdentityCredentialFromComponents(td, "/workload")
	selectors := domain.NewSelectorSet()
	sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
	selectors.Add(sel)

	tests := []struct {
		name         string
		mapper       *domain.IdentityMapper
		input        *domain.SelectorSet
		expectMatch  bool
		description  string
	}{
		{
			name:         "nil mapper returns false",
			mapper:       nil,
			input:        selectors,
			expectMatch:  false,
			description:  "Nil receiver should be safe and return false",
		},
		{
			name: "valid mapper with nil input returns false",
			mapper: func() *domain.IdentityMapper {
				m, _ := domain.NewIdentityMapper(id, selectors)
				return m
			}(),
			input:        nil,
			expectMatch:  false,
			description:  "Nil input should return false",
		},
		{
			name: "valid mapper with valid input matches",
			mapper: func() *domain.IdentityMapper {
				m, _ := domain.NewIdentityMapper(id, selectors)
				return m
			}(),
			input:        selectors,
			expectMatch:  true,
			description:  "Valid inputs should match when selectors are present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			matches := tt.mapper.MatchesSelectors(tt.input)

			// Assert
			assert.Equal(t, tt.expectMatch, matches, tt.description)
		})
	}
}

// TestIdentityMapper_SetParentID tests setting parent identity
func TestIdentityMapper_SetParentID(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")
	id := domain.NewIdentityCredentialFromComponents(td, "/workload")
	selectors := domain.NewSelectorSet()
	sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
	selectors.Add(sel)

	mapper, err := domain.NewIdentityMapper(id, selectors)
	require.NoError(t, err)

	// Initially parent should be nil
	assert.Nil(t, mapper.ParentID())

	// Act - Set parent ID
	parentID := domain.NewIdentityCredentialFromComponents(td, "/spire/agent/node-1")
	mapper.SetParentID(parentID)

	// Assert
	assert.NotNil(t, mapper.ParentID())
	assert.Equal(t, parentID, mapper.ParentID())
	assert.Equal(t, "spiffe://example.org/spire/agent/node-1", mapper.ParentID().String())
}

// TestIdentityMapper_Accessors tests all accessor methods
func TestIdentityMapper_Accessors(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")
	id := domain.NewIdentityCredentialFromComponents(td, "/workload/db")
	selectors := domain.NewSelectorSet()
	sel1, _ := domain.ParseSelectorFromString("unix:uid:1000")
	sel2, _ := domain.ParseSelectorFromString("k8s:namespace:prod")
	selectors.Add(sel1)
	selectors.Add(sel2)

	mapper, err := domain.NewIdentityMapper(id, selectors)
	require.NoError(t, err)

	// Act & Assert - IdentityCredential()
	assert.Equal(t, id, mapper.IdentityCredential())
	assert.Equal(t, "spiffe://example.org/workload/db", mapper.IdentityCredential().String())

	// Act & Assert - Selectors()
	assert.NotNil(t, mapper.Selectors())
	assert.Equal(t, 2, len(mapper.Selectors().All()))
	assert.True(t, mapper.Selectors().Contains(sel1))
	assert.True(t, mapper.Selectors().Contains(sel2))

	// Act & Assert - ParentID() (initially nil)
	assert.Nil(t, mapper.ParentID())
}

// TestIdentityMapper_MatchesSelectors_EmptyInput tests matching with empty selector set
func TestIdentityMapper_MatchesSelectors_EmptyInput(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")
	id := domain.NewIdentityCredentialFromComponents(td, "/workload")
	selectors := domain.NewSelectorSet()
	sel, _ := domain.ParseSelectorFromString("unix:uid:1000")
	selectors.Add(sel)

	mapper, err := domain.NewIdentityMapper(id, selectors)
	require.NoError(t, err)

	// Act - Empty selector set
	emptySet := domain.NewSelectorSet()
	matches := mapper.MatchesSelectors(emptySet)

	// Assert - Should not match (missing required selector)
	assert.False(t, matches, "Empty selector set should not match mapper requiring selectors")
}
