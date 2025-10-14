package domain_test

// TrustDomain Invariant Tests
//
// These tests verify domain invariants for the TrustDomain value object.
// Invariants are properties that must always hold true, regardless of input.
//
// Run these tests with:
//
//	go test ./internal/domain/... -v -run TestTrustDomain_Invariant

import (
	"testing"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
)

// TestTrustDomain_Invariant_StringNeverEmpty tests the invariants:
// - "Name is never empty after construction"
// - "String() never returns empty string for valid TrustDomain"
func TestTrustDomain_Invariant_StringNeverEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputName  string
		expectName string
	}{
		{"standard domain", "example.org", "example.org"},
		{"subdomain", "prod.example.org", "prod.example.org"},
		{"single char", "x", "x"},
		{"with dash", "my-domain.com", "my-domain.com"},
		{"uppercase input", "Example.ORG", "example.org"}, // Canonicalized to lowercase
		{"mixed case", "Prod.Example.Org", "prod.example.org"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			td := domain.NewTrustDomainFromName(tt.inputName)

			// Assert invariant: String() never returns empty
			assert.NotNil(t, td)
			str := td.String()
			assert.NotEmpty(t, str, "Invariant violated: String() returned empty string")
			assert.Greater(t, len(str), 0, "Invariant violated: String() length must be > 0")
			assert.Equal(t, tt.expectName, str, "Should return canonical lowercase form")
		})
	}
}

// TestTrustDomain_Invariant_EqualsNilSafeAndReflexive tests the invariants:
// - "Equals() returns false for nil input, never panics"
// - "Equals() is reflexive: td.Equals(td) == true"
func TestTrustDomain_Invariant_EqualsNilSafeAndReflexive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		td   *domain.TrustDomain
	}{
		{"simple name", domain.NewTrustDomainFromName("example.org")},
		{"subdomain", domain.NewTrustDomainFromName("prod.example.org")},
		{"single char", domain.NewTrustDomainFromName("x")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Assert invariant: Equals(nil) returns false, never panics
			assert.NotPanics(t, func() {
				result := tt.td.Equals(nil)
				assert.False(t, result, "Invariant violated: Equals(nil) should return false")
			})

			// Assert invariant: reflexive property
			assert.True(t, tt.td.Equals(tt.td), "Invariant violated: Equals() is not reflexive")
		})
	}
}

// TestTrustDomain_Invariant_EqualsSymmetric tests the invariant:
// "Equals() is symmetric: td1.Equals(td2) == td2.Equals(td1)"
func TestTrustDomain_Invariant_EqualsSymmetric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		name1  string
		name2  string
		equals bool
	}{
		{"same names", "example.org", "example.org", true},
		{"different names", "example.org", "other.org", false},
		{"case difference", "example.org", "Example.org", true}, // Case-insensitive (canonical lowercase)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td1 := domain.NewTrustDomainFromName(tt.name1)
			td2 := domain.NewTrustDomainFromName(tt.name2)

			// Assert invariant: symmetric property
			assert.Equal(t, td1.Equals(td2), td2.Equals(td1),
				"Invariant violated: Equals() is not symmetric")
			assert.Equal(t, tt.equals, td1.Equals(td2))
		})
	}
}

// TestTrustDomain_Invariant_EqualsTransitive tests the invariant:
// "Equals() is transitive: if td1.Equals(td2) && td2.Equals(td3), then td1.Equals(td3)"
func TestTrustDomain_Invariant_EqualsTransitive(t *testing.T) {
	t.Parallel()

	// Arrange
	td1 := domain.NewTrustDomainFromName("example.org")
	td2 := domain.NewTrustDomainFromName("example.org")
	td3 := domain.NewTrustDomainFromName("example.org")

	// Assert invariant: transitive property
	assert.True(t, td1.Equals(td2), "Setup: td1 should equal td2")
	assert.True(t, td2.Equals(td3), "Setup: td2 should equal td3")
	assert.True(t, td1.Equals(td3), "Invariant violated: Equals() is not transitive")
}

// TestTrustDomain_Invariant_IsZero tests the invariants:
// - "IsZero() returns true for nil trust domain, false otherwise"
// - "IsZero() never panics, even on nil receiver"
// - "Constructor panics on empty string (defensive check)"
func TestTrustDomain_Invariant_IsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		td         *domain.TrustDomain
		expectZero bool
	}{
		{"nil trust domain", nil, true},
		{"valid domain", domain.NewTrustDomainFromName("example.org"), false},
		{"single char", domain.NewTrustDomainFromName("x"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Assert invariant: IsZero() never panics
			assert.NotPanics(t, func() {
				result := tt.td.IsZero()

				// Assert invariant: IsZero() correctly identifies uninitialized state
				assert.Equal(t, tt.expectZero, result,
					"Invariant violated: IsZero() returned unexpected value")
			}, "Invariant violated: IsZero() panicked")
		})
	}
}

// TestTrustDomain_Invariant_ConstructorPanicsOnEmpty tests the invariant:
// "Constructor panics on empty string to prevent invalid state"
func TestTrustDomain_Invariant_ConstructorPanicsOnEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tabs only", "\t\t"},
		{"newlines", "\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Assert invariant: constructor panics on empty/whitespace
			assert.Panics(t, func() {
				domain.NewTrustDomainFromName(tt.input)
			}, "Invariant violated: constructor should panic on empty/whitespace input")
		})
	}
}

// TestTrustDomain_Invariant_CompareOrdering tests the invariants:
// "Compare provides deterministic ordering and is nil-safe"
func TestTrustDomain_Invariant_CompareOrdering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		td1      *domain.TrustDomain
		td2      *domain.TrustDomain
		expected int
	}{
		{"nil == nil", nil, nil, 0},
		{"nil < non-nil", nil, domain.NewTrustDomainFromName("example.org"), -1},
		{"non-nil > nil", domain.NewTrustDomainFromName("example.org"), nil, 1},
		{"equal domains", domain.NewTrustDomainFromName("example.org"), domain.NewTrustDomainFromName("example.org"), 0},
		{"a < z", domain.NewTrustDomainFromName("aaa.org"), domain.NewTrustDomainFromName("zzz.org"), -1},
		{"z > a", domain.NewTrustDomainFromName("zzz.org"), domain.NewTrustDomainFromName("aaa.org"), 1},
		{"case insensitive", domain.NewTrustDomainFromName("Example.ORG"), domain.NewTrustDomainFromName("example.org"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Assert invariant: Compare never panics
			assert.NotPanics(t, func() {
				result := tt.td1.Compare(tt.td2)
				assert.Equal(t, tt.expected, result,
					"Invariant violated: Compare returned unexpected ordering")
			}, "Invariant violated: Compare panicked")
		})
	}
}

// TestTrustDomain_Invariant_StringNilSafe tests the invariant:
// "String() never panics, even on nil receiver"
func TestTrustDomain_Invariant_StringNilSafe(t *testing.T) {
	t.Parallel()

	var nilTD *domain.TrustDomain

	// Assert invariant: String() on nil returns empty, never panics
	assert.NotPanics(t, func() {
		result := nilTD.String()
		assert.Equal(t, "", result, "Invariant violated: nil.String() should return empty string")
	}, "Invariant violated: String() panicked on nil receiver")
}

// TestTrustDomain_Invariant_KeyStability tests the invariant:
// "Key() returns stable string for use in maps/sets"
func TestTrustDomain_Invariant_KeyStability(t *testing.T) {
	t.Parallel()

	td := domain.NewTrustDomainFromName("example.org")

	// Call Key() multiple times
	key1 := td.Key()
	key2 := td.Key()
	key3 := td.Key()

	// Assert invariant: Key() returns same value on repeated calls
	assert.Equal(t, key1, key2, "Invariant violated: Key() is not stable")
	assert.Equal(t, key2, key3, "Invariant violated: Key() is not stable")
	assert.Equal(t, "example.org", key1, "Key should match canonical lowercase form")
}

// TestTrustDomain_Invariant_MarshalTextRejectsInvalid tests the invariant:
// "MarshalText returns error for nil or empty trust domain"
func TestTrustDomain_Invariant_MarshalTextRejectsInvalid(t *testing.T) {
	t.Parallel()

	var nilTD *domain.TrustDomain

	// Assert invariant: MarshalText on nil returns error
	_, err := nilTD.MarshalText()
	assert.Error(t, err, "Invariant violated: MarshalText should error on nil")

	// Valid trust domain should marshal successfully
	validTD := domain.NewTrustDomainFromName("example.org")
	b, err := validTD.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("example.org"), b)
}

// TestTrustDomain_Invariant_MarshalJSONRejectsInvalid tests the invariant:
// "MarshalJSON returns error for nil or empty trust domain"
func TestTrustDomain_Invariant_MarshalJSONRejectsInvalid(t *testing.T) {
	t.Parallel()

	var nilTD *domain.TrustDomain

	// Assert invariant: MarshalJSON on nil returns error
	_, err := nilTD.MarshalJSON()
	assert.Error(t, err, "Invariant violated: MarshalJSON should error on nil")

	// Valid trust domain should marshal successfully
	validTD := domain.NewTrustDomainFromName("example.org")
	b, err := validTD.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, []byte(`"example.org"`), b)
}
