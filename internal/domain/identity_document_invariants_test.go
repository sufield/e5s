package domain_test

import (
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIdentityDocument_Invariant_NamespaceNeverNil tests the invariant:
// "identityNamespace is never nil for valid document"
func TestIdentityDocument_Invariant_NamespaceNeverNil(t *testing.T) {
	t.Parallel()

	// Arrange - Use fixed time for determinism
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
	expiresAt := time.Unix(2000000000, 0) // May 18, 2033 - fixed future time

	// Act
	doc := domain.NewIdentityDocumentFromComponents(
		namespace,
		domain.IdentityDocumentTypeX509,
		nil, // cert
		nil, // privateKey
		nil, // chain
		expiresAt,
	)

	// Assert invariant: identityNamespace is never nil
	require.NotNil(t, doc)
	assert.NotNil(t, doc.IdentityNamespace(),
		"Invariant violated: IdentityNamespace() returned nil")
	assert.Equal(t, namespace, doc.IdentityNamespace())
}

// TestIdentityDocument_Invariant_IsExpiredMonotonic tests the invariant:
// "IsExpired() transitions from false → true, never true → false"
func TestIdentityDocument_Invariant_IsExpiredMonotonic(t *testing.T) {
	t.Parallel()

	// Arrange - Use fixed time for determinism
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")

	// Use Unix epoch time for deterministic testing
	// expiresAt is set to a specific past time
	expiresAt := time.Unix(1000000000, 0) // January 9, 2001 - definitely in the past

	doc := domain.NewIdentityDocumentFromComponents(
		namespace,
		domain.IdentityDocumentTypeX509,
		nil, nil, nil,
		expiresAt,
	)

	// Assert invariant: document expired (since expiresAt is in the past)
	firstCheck := doc.IsExpired()
	assert.True(t, firstCheck, "Document with past expiry should be expired")

	// Check again - should still be expired (monotonic: once true, stays true)
	secondCheck := doc.IsExpired()
	assert.True(t, secondCheck,
		"Invariant violated: IsExpired() should never transition from true to false")

	// Third check confirms monotonicity
	thirdCheck := doc.IsExpired()
	assert.True(t, thirdCheck, "Expiration state should remain true")

	// Verify all checks returned same value (true)
	assert.Equal(t, firstCheck, secondCheck, "IsExpired() should be consistent")
	assert.Equal(t, secondCheck, thirdCheck, "IsExpired() should be consistent")
}

// TestIdentityDocument_Invariant_IsValidEqualsNotExpired tests the invariant:
// "IsValid() == !IsExpired() for current implementation"
func TestIdentityDocument_Invariant_IsValidEqualsNotExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		expiresAt time.Time
	}{
		{"valid document", time.Unix(2000000000, 0)}, // May 18, 2033 - future
		{"expired document", time.Unix(1000000000, 0)}, // January 9, 2001 - past
		{"just expired", time.Unix(1500000000, 0)}, // July 14, 2017 - past
		{"expires far future", time.Unix(2500000000, 0)}, // March 3, 2049 - far future
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName("example.org")
			namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
			doc := domain.NewIdentityDocumentFromComponents(
				namespace,
				domain.IdentityDocumentTypeX509,
				nil, nil, nil,
				tt.expiresAt,
			)

			// Assert invariant: IsValid() == !IsExpired()
			assert.Equal(t, !doc.IsExpired(), doc.IsValid(),
				"Invariant violated: IsValid() must equal !IsExpired()")
		})
	}
}

// TestIdentityDocument_Invariant_ExpirationTimeCheck tests the invariant:
// "IsExpired() iff time.Now().After(expiresAt)"
func TestIdentityDocument_Invariant_ExpirationTimeCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		expiresAt      time.Time
		expectedExpired bool
	}{
		{"future expiration", time.Unix(2000000000, 0), false}, // May 18, 2033
		{"past expiration", time.Unix(1000000000, 0), true},    // January 9, 2001
		{"very far future", time.Unix(2500000000, 0), false},   // March 3, 2049
		{"just expired", time.Unix(1500000000, 0), true},       // July 14, 2017
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName("example.org")
			namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
			doc := domain.NewIdentityDocumentFromComponents(
				namespace,
				domain.IdentityDocumentTypeX509,
				nil, nil, nil,
				tt.expiresAt,
			)

			// Assert invariant: IsExpired() matches time check
			currentlyExpired := time.Now().After(tt.expiresAt)
			assert.Equal(t, currentlyExpired, doc.IsExpired(),
				"Invariant violated: IsExpired() must match time.Now().After(expiresAt)")
			assert.Equal(t, tt.expectedExpired, doc.IsExpired())
		})
	}
}

// TestIdentityDocument_Invariant_TypePreservation tests the invariant:
// "Document type is preserved after construction"
func TestIdentityDocument_Invariant_TypePreservation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		docType  domain.IdentityDocumentType
	}{
		{"X509 type", domain.IdentityDocumentTypeX509},
		{"JWT type", domain.IdentityDocumentTypeJWT},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName("example.org")
			namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
			expiresAt := time.Unix(2000000000, 0) // May 18, 2033 - fixed future time

			// Act
			doc := domain.NewIdentityDocumentFromComponents(
				namespace,
				tt.docType,
				nil, nil, nil,
				expiresAt,
			)

			// Assert invariant: type is preserved
			assert.Equal(t, tt.docType, doc.Type(),
				"Invariant violated: document type was not preserved")
		})
	}
}

// TestIdentityDocument_Invariant_Immutability tests the invariant:
// "Identity documents are immutable after creation"
func TestIdentityDocument_Invariant_Immutability(t *testing.T) {
	t.Parallel()

	// Arrange - Use fixed time for determinism
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
	expiresAt := time.Unix(2000000000, 0) // May 18, 2033 - fixed future time

	doc := domain.NewIdentityDocumentFromComponents(
		namespace,
		domain.IdentityDocumentTypeX509,
		nil, nil, nil,
		expiresAt,
	)

	// Store initial values
	initialNamespace := doc.IdentityNamespace()
	initialType := doc.Type()
	initialExpiresAt := doc.ExpiresAt()

	// Call getters multiple times (no time.Sleep needed - using fixed time)
	// If document were mutable, values could change between calls
	secondNamespace := doc.IdentityNamespace()
	secondType := doc.Type()
	secondExpiresAt := doc.ExpiresAt()

	// Assert invariant: all fields unchanged (immutable)
	assert.Equal(t, initialNamespace, secondNamespace,
		"Invariant violated: identityNamespace was modified")
	assert.Equal(t, initialType, secondType,
		"Invariant violated: type was modified")
	assert.Equal(t, initialExpiresAt, secondExpiresAt,
		"Invariant violated: expiresAt was modified")

	// Verify exact values match construction parameters
	assert.Equal(t, namespace, doc.IdentityNamespace())
	assert.Equal(t, domain.IdentityDocumentTypeX509, doc.Type())
	assert.Equal(t, expiresAt, doc.ExpiresAt())
}

// TestIdentityDocument_Invariant_JWTHasNilCrypto tests the invariant:
// "For JWT documents, cert/privateKey/chain are nil"
func TestIdentityDocument_Invariant_JWTHasNilCrypto(t *testing.T) {
	t.Parallel()

	// Arrange - Use fixed time for determinism
	td := domain.NewTrustDomainFromName("example.org")
	namespace := domain.NewIdentityNamespaceFromComponents(td, "/workload")
	expiresAt := time.Unix(2000000000, 0) // May 18, 2033 - fixed future time

	// Act - Create JWT document
	doc := domain.NewIdentityDocumentFromComponents(
		namespace,
		domain.IdentityDocumentTypeJWT,
		nil, // cert should be nil for JWT
		nil, // privateKey should be nil for JWT
		nil, // chain should be nil for JWT
		expiresAt,
	)

	// Assert invariant: JWT documents have nil crypto material
	assert.Nil(t, doc.Certificate(),
		"Invariant violated: JWT document should have nil Certificate")
	assert.Nil(t, doc.PrivateKey(),
		"Invariant violated: JWT document should have nil PrivateKey")
	assert.Nil(t, doc.Chain(),
		"Invariant violated: JWT document should have nil Chain")
	assert.Equal(t, domain.IdentityDocumentTypeJWT, doc.Type())
}
