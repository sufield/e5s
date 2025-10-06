package domain_test

import (
	"testing"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
)

// TestTrustDomain_Invariant_NameNeverEmpty tests the invariant:
// "Name is never empty after construction"
func TestTrustDomain_Invariant_NameNeverEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		inputName      string
		expectNonEmpty bool
	}{
		{
			name:           "valid trust domain name",
			inputName:      "example.org",
			expectNonEmpty: true,
		},
		{
			name:           "valid trust domain with subdomain",
			inputName:      "prod.example.org",
			expectNonEmpty: true,
		},
		{
			name:           "single character name",
			inputName:      "x",
			expectNonEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			td := domain.NewTrustDomainFromName(tt.inputName)

			// Assert invariant: name is never empty
			assert.NotNil(t, td)
			assert.NotEmpty(t, td.String(), "Invariant violated: TrustDomain.String() returned empty string")
			assert.Equal(t, tt.inputName, td.String())
		})
	}
}

// TestTrustDomain_Invariant_EqualsNilSafe tests the invariant:
// "Equals() returns false for nil input, never panics"
func TestTrustDomain_Invariant_EqualsNilSafe(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")

	// Act & Assert invariant: Equals(nil) returns false, never panics
	assert.NotPanics(t, func() {
		result := td.Equals(nil)
		assert.False(t, result, "Invariant violated: Equals(nil) should return false")
	})
}

// TestTrustDomain_Invariant_EqualsReflexive tests the invariant:
// "Equals() is reflexive: td.Equals(td) == true"
func TestTrustDomain_Invariant_EqualsReflexive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		inputName string
	}{
		{"simple name", "example.org"},
		{"subdomain", "prod.example.org"},
		{"single char", "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName(tt.inputName)

			// Assert invariant: reflexive property
			assert.True(t, td.Equals(td), "Invariant violated: Equals() is not reflexive")
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

// TestTrustDomain_Invariant_StringNeverEmpty tests the invariant:
// "String() never returns empty string for valid TrustDomain"
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

			// Arrange
			td := domain.NewTrustDomainFromName(tt.inputName)

			// Assert invariant: String() never returns empty
			str := td.String()
			assert.NotEmpty(t, str, "Invariant violated: String() returned empty string")
			assert.Greater(t, len(str), 0, "Invariant violated: String() length must be > 0")
		})
	}
}
