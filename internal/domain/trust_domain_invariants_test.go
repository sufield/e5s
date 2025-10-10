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
		name      string
		inputName string
	}{
		{"standard domain", "example.org"},
		{"subdomain", "prod.example.org"},
		{"single char", "x"},
		{"with dash", "my-domain.com"},
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
			assert.Equal(t, tt.inputName, str)
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
		{"case difference", "example.org", "Example.org", false}, // Case-sensitive
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
// - "IsZero() returns true for nil or empty trust domain, false otherwise"
// - "IsZero() never panics, even on nil receiver"
func TestTrustDomain_Invariant_IsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		td         *domain.TrustDomain
		expectZero bool
	}{
		{"nil trust domain", nil, true},
		{"empty name", domain.NewTrustDomainFromName(""), true},
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
