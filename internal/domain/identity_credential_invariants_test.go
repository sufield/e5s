package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// TestIdentityCredential_Invariant_TrustDomainNeverNil tests the invariant:
// "trustDomain is never nil after construction"
func TestIdentityCredential_Invariant_TrustDomainNeverNil(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		trustDomain string
		path        string
	}{
		{"with path", "example.org", "/workload"},
		{"root path", "example.org", "/"},
		{"empty path defaults to /", "example.org", ""},
		{"nested path", "prod.example.org", "/ns/prod/svc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName(tt.trustDomain)

			// Act
			id := domain.NewIdentityCredentialFromComponents(td, tt.path)

			// Assert invariant: trustDomain is never nil
			require.NotNil(t, id)
			assert.NotNil(t, id.TrustDomain(), "Invariant violated: TrustDomain() returned nil")
			assert.Equal(t, td, id.TrustDomain())
		})
	}
}

// TestIdentityCredential_Invariant_PathNeverEmpty tests the invariant:
// "path defaults to '/' if empty, never stored as empty string"
func TestIdentityCredential_Invariant_PathNeverEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputPath    string
		expectedPath string
	}{
		{"empty path defaults to /", "", "/"},
		{"explicit root path", "/", "/"},
		{"single segment", "/workload", "/workload"},
		{"nested path", "/ns/prod/svc", "/ns/prod/svc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName("example.org")

			// Act
			id := domain.NewIdentityCredentialFromComponents(td, tt.inputPath)

			// Assert invariant: path is never empty
			path := id.Path()
			assert.NotEmpty(t, path, "Invariant violated: Path() returned empty string")
			assert.Equal(t, tt.expectedPath, path)
		})
	}
}

// TestIdentityCredential_Invariant_URIFormat tests the invariant:
// "uri is always formatted as 'spiffe://<trustDomain><path>'"
func TestIdentityCredential_Invariant_URIFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		trustDomain string
		path        string
		expectedURI string
	}{
		{
			name:        "standard format",
			trustDomain: "example.org",
			path:        "/workload",
			expectedURI: "spiffe://example.org/workload",
		},
		{
			name:        "root path",
			trustDomain: "example.org",
			path:        "/",
			expectedURI: "spiffe://example.org/",
		},
		{
			name:        "empty path becomes root",
			trustDomain: "example.org",
			path:        "",
			expectedURI: "spiffe://example.org/",
		},
		{
			name:        "nested path",
			trustDomain: "prod.example.org",
			path:        "/ns/prod/svc/api",
			expectedURI: "spiffe://prod.example.org/ns/prod/svc/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName(tt.trustDomain)

			// Act
			id := domain.NewIdentityCredentialFromComponents(td, tt.path)

			// Assert invariant: URI format is "spiffe://<trustDomain><path>"
			uri := id.String()
			assert.NotEmpty(t, uri, "Invariant violated: String() returned empty")
			assert.Contains(t, uri, "spiffe://", "Invariant violated: URI must start with 'spiffe://'")
			assert.Equal(t, tt.expectedURI, uri)

			// Verify URI matches components
			expectedPath := tt.path
			if expectedPath == "" {
				expectedPath = "/"
			}
			assert.Equal(t, "spiffe://"+tt.trustDomain+expectedPath, uri,
				"Invariant violated: URI must match trustDomain + path")
		})
	}
}

// TestIdentityCredential_Invariant_EqualsReflexive tests the invariant:
// "Equals() is reflexive"
func TestIdentityCredential_Invariant_EqualsReflexive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		trustDomain string
		path        string
	}{
		{"simple", "example.org", "/workload"},
		{"root", "example.org", "/"},
		{"nested", "prod.example.org", "/ns/prod/svc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName(tt.trustDomain)
			id := domain.NewIdentityCredentialFromComponents(td, tt.path)

			// Assert invariant: reflexive
			//nolint:gocritic // Intentional duplicate for testing Equals reflexivity
			assert.True(t, id.Equals(id), "Invariant violated: Equals() is not reflexive")
		})
	}
}

// TestIdentityCredential_Invariant_EqualsSymmetric tests the invariant:
// "Equals() is symmetric"
func TestIdentityCredential_Invariant_EqualsSymmetric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		td1    string
		path1  string
		td2    string
		path2  string
		equals bool
	}{
		{"same credential", "example.org", "/workload", "example.org", "/workload", true},
		{"different paths", "example.org", "/workload", "example.org", "/service", false},
		{"different domains", "example.org", "/workload", "other.org", "/workload", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			id1 := domain.NewIdentityCredentialFromComponents(
				domain.NewTrustDomainFromName(tt.td1), tt.path1)
			id2 := domain.NewIdentityCredentialFromComponents(
				domain.NewTrustDomainFromName(tt.td2), tt.path2)

			// Assert invariant: symmetric
			assert.Equal(t, id1.Equals(id2), id2.Equals(id1),
				"Invariant violated: Equals() is not symmetric")
			assert.Equal(t, tt.equals, id1.Equals(id2))
		})
	}
}

// TestIdentityCredential_Invariant_EqualsTransitive tests the invariant:
// "Equals() is transitive"
func TestIdentityCredential_Invariant_EqualsTransitive(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")
	id1 := domain.NewIdentityCredentialFromComponents(td, "/workload")
	id2 := domain.NewIdentityCredentialFromComponents(td, "/workload")
	id3 := domain.NewIdentityCredentialFromComponents(td, "/workload")

	// Assert invariant: transitive
	assert.True(t, id1.Equals(id2), "Setup: id1 should equal id2")
	assert.True(t, id2.Equals(id3), "Setup: id2 should equal id3")
	assert.True(t, id1.Equals(id3), "Invariant violated: Equals() is not transitive")
}

// TestIdentityCredential_Invariant_EqualsNilSafe tests the invariant:
// "Equals() returns false for nil input, never panics"
func TestIdentityCredential_Invariant_EqualsNilSafe(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")
	id := domain.NewIdentityCredentialFromComponents(td, "/workload")

	// Assert invariant: nil-safe
	assert.NotPanics(t, func() {
		result := id.Equals(nil)
		assert.False(t, result, "Invariant violated: Equals(nil) should return false")
	})
}

// TestIdentityCredential_Invariant_IsInTrustDomain tests the invariant:
// "IsInTrustDomain(td) iff i.trustDomain.Equals(td)"
func TestIdentityCredential_Invariant_IsInTrustDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		idTrustDomain  string
		idPath         string
		checkDomain    string
		expectedResult bool
	}{
		{
			name:           "same trust domain",
			idTrustDomain:  "example.org",
			idPath:         "/workload",
			checkDomain:    "example.org",
			expectedResult: true,
		},
		{
			name:           "different trust domain",
			idTrustDomain:  "example.org",
			idPath:         "/workload",
			checkDomain:    "other.org",
			expectedResult: false,
		},
		{
			name:           "subdomain is different",
			idTrustDomain:  "prod.example.org",
			idPath:         "/svc",
			checkDomain:    "example.org",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName(tt.idTrustDomain)
			id := domain.NewIdentityCredentialFromComponents(td, tt.idPath)
			checkTD := domain.NewTrustDomainFromName(tt.checkDomain)

			// Act
			result := id.IsInTrustDomain(checkTD)

			// Assert invariant: IsInTrustDomain matches trustDomain.Equals
			assert.Equal(t, tt.expectedResult, result)
			assert.Equal(t, id.TrustDomain().Equals(checkTD), result,
				"Invariant violated: IsInTrustDomain must match trustDomain.Equals")
		})
	}
}
