package domain_test

import (
	"testing"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantType  domain.SelectorType
		wantKey   string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "valid unix uid selector",
			input:     "unix:uid:1000",
			wantType:  domain.SelectorType("unix"),
			wantKey:   "uid",
			wantValue: "1000",
			wantErr:   false,
		},
		{
			name:      "valid k8s namespace selector",
			input:     "k8s:namespace:prod",
			wantType:  domain.SelectorType("k8s"),
			wantKey:   "namespace",
			wantValue: "prod",
			wantErr:   false,
		},
		{
			name:      "valid selector with complex value",
			input:     "custom:key:value:with:colons",
			wantType:  domain.SelectorType("custom"),
			wantKey:   "key",
			wantValue: "value:with:colons",
			wantErr:   false,
		},
		{
			name:      "invalid selector missing colon",
			input:     "invalid",
			wantType:  "",
			wantValue: "",
			wantErr:   true,
		},
		{
			name:      "invalid selector empty type",
			input:     ":value",
			wantType:  "",
			wantValue: "",
			wantErr:   true,
		},
		{
			name:      "invalid selector empty value",
			input:     "type:",
			wantType:  "",
			wantValue: "",
			wantErr:   true,
		},
		{
			name:      "invalid empty selector",
			input:     "",
			wantType:  "",
			wantValue: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			selector, err := domain.ParseSelectorFromString(tt.input)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, selector)
			} else {
				require.NoError(t, err)
				require.NotNil(t, selector)
				assert.Equal(t, tt.wantType, selector.Type())
				assert.Equal(t, tt.wantKey, selector.Key())
				assert.Equal(t, tt.wantValue, selector.Value())
				assert.Equal(t, tt.input, selector.String())
			}
		})
	}
}

func TestSelector_Equals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		sel1   *domain.Selector
		sel2   *domain.Selector
		equals bool
	}{
		{
			name:   "equal selectors",
			sel1:   mustParseSelector(t, "unix:uid:1000"),
			sel2:   mustParseSelector(t, "unix:uid:1000"),
			equals: true,
		},
		{
			name:   "different values",
			sel1:   mustParseSelector(t, "unix:uid:1000"),
			sel2:   mustParseSelector(t, "unix:uid:1001"),
			equals: false,
		},
		{
			name:   "different types",
			sel1:   mustParseSelector(t, "unix:uid:1000"),
			sel2:   mustParseSelector(t, "k8s:uid:1000"),
			equals: false,
		},
		{
			name:   "nil comparison",
			sel1:   mustParseSelector(t, "unix:uid:1000"),
			sel2:   nil,
			equals: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := tt.sel1.Equals(tt.sel2)

			// Assert
			assert.Equal(t, tt.equals, result)
		})
	}
}

func TestSelectorSet_AddAndContains(t *testing.T) {
	t.Parallel()

	// Arrange
	set := domain.NewSelectorSet()
	sel1 := mustParseSelector(t, "unix:uid:1000")
	sel2 := mustParseSelector(t, "unix:gid:1000")

	// Act & Assert - Initially empty
	assert.False(t, set.Contains(sel1))

	// Add selector
	set.Add(sel1)
	assert.True(t, set.Contains(sel1))
	assert.False(t, set.Contains(sel2))

	// Add second selector
	set.Add(sel2)
	assert.True(t, set.Contains(sel1))
	assert.True(t, set.Contains(sel2))

	// Adding duplicate does not change set
	set.Add(sel1)
	assert.True(t, set.Contains(sel1))
}

// TestSelectorSet_MultipleSelectors tests adding multiple selectors
func TestSelectorSet_MultipleSelectors(t *testing.T) {
	t.Parallel()

	// Arrange
	set := domain.NewSelectorSet()
	sel1 := mustParseSelector(t, "unix:uid:1000")
	sel2 := mustParseSelector(t, "unix:gid:1000")
	sel3 := mustParseSelector(t, "k8s:ns:prod")

	// Act - Add multiple selectors
	set.Add(sel1)
	set.Add(sel2)
	set.Add(sel3)

	// Assert - All selectors are in the set
	assert.True(t, set.Contains(sel1))
	assert.True(t, set.Contains(sel2))
	assert.True(t, set.Contains(sel3))

	// Assert - Non-existent selector is not in set
	sel4 := mustParseSelector(t, "unix:uid:2000")
	assert.False(t, set.Contains(sel4))
}

// Helper function to parse selector or fail test
func mustParseSelector(t *testing.T, s string) *domain.Selector {
	t.Helper()
	sel, err := domain.ParseSelectorFromString(s)
	require.NoError(t, err)
	return sel
}
