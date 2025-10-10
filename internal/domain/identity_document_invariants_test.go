package domain_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIdentityDocument_Invariant_NamespaceNeverNil tests the invariant:
// "identityCredential is never nil for valid document"
func TestIdentityDocument_Invariant_NamespaceNeverNil(t *testing.T) {
	t.Parallel()

	// Arrange - Use fixed time for determinism
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
	expiresAt := time.Unix(2000000000, 0) // May 18, 2033 - fixed future time

	// Act
	doc := domain.NewIdentityDocumentFromComponents(
		credential,
		nil, // cert
		nil, // privateKey
		nil, // chain
		expiresAt,
	)

	// Assert invariant: identityCredential is never nil
	require.NotNil(t, doc)
	assert.NotNil(t, doc.IdentityCredential(),
		"Invariant violated: IdentityCredential() returned nil")
	assert.Equal(t, credential, doc.IdentityCredential())
}

// TestIdentityDocument_Invariant_IsExpiredMonotonic tests the invariant:
// "IsExpired() transitions from false → true, never true → false"
func TestIdentityDocument_Invariant_IsExpiredMonotonic(t *testing.T) {
	t.Parallel()

	// Arrange - Use fixed time for determinism
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")

	// Use Unix epoch time for deterministic testing
	// expiresAt is set to a specific past time
	expiresAt := time.Unix(1000000000, 0) // January 9, 2001 - definitely in the past

	doc := domain.NewIdentityDocumentFromComponents(
		credential,
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
		{"valid document", time.Unix(2000000000, 0)},     // May 18, 2033 - future
		{"expired document", time.Unix(1000000000, 0)},   // January 9, 2001 - past
		{"just expired", time.Unix(1500000000, 0)},       // July 14, 2017 - past
		{"expires far future", time.Unix(2500000000, 0)}, // March 3, 2049 - far future
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			td := domain.NewTrustDomainFromName("example.org")
			credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
			doc := domain.NewIdentityDocumentFromComponents(
				credential,
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
		name            string
		expiresAt       time.Time
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
			credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
			doc := domain.NewIdentityDocumentFromComponents(
				credential,
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

// TestIdentityDocument_Invariant_Immutability tests the invariant:
// "Identity documents are immutable after creation"
func TestIdentityDocument_Invariant_Immutability(t *testing.T) {
	t.Parallel()

	// Arrange - Use fixed time for determinism
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
	expiresAt := time.Unix(2000000000, 0) // May 18, 2033 - fixed future time

	doc := domain.NewIdentityDocumentFromComponents(
		credential,
		nil, nil, nil,
		expiresAt,
	)

	// Store initial values
	initialNamespace := doc.IdentityCredential()
	initialExpiresAt := doc.ExpiresAt()

	// Call getters multiple times (no time.Sleep needed - using fixed time)
	// If document were mutable, values could change between calls
	secondNamespace := doc.IdentityCredential()
	secondExpiresAt := doc.ExpiresAt()

	// Assert invariant: all fields unchanged (immutable)
	assert.Equal(t, initialNamespace, secondNamespace,
		"Invariant violated: identityCredential was modified")
	assert.Equal(t, initialExpiresAt, secondExpiresAt,
		"Invariant violated: expiresAt was modified")

	// Verify exact values match construction parameters
	assert.Equal(t, credential, doc.IdentityCredential())
	assert.Equal(t, expiresAt, doc.ExpiresAt())
}

// TestIdentityDocument_Invariant_X509ComponentsNonNil tests the invariant:
// "Certificate, PrivateKey, and Chain are non-nil for valid X.509 documents"
func TestIdentityDocument_Invariant_X509ComponentsNonNil(t *testing.T) {
	t.Parallel()

	// Arrange - Create valid X.509 document with real certificate components
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
	expiresAt := time.Unix(2000000000, 0) // May 18, 2033 - fixed future time

	// Create a mock certificate for testing X.509 components
	// Note: In real usage, this would come from SPIRE or in-memory provider
	mockCert := generateMockCertificate(t)
	mockPrivateKey := generateMockPrivateKey(t)
	mockChain := []*x509.Certificate{mockCert}

	// Act - Create document with X.509 components
	doc := domain.NewIdentityDocumentFromComponents(
		credential,
		mockCert,
		mockPrivateKey,
		mockChain,
		expiresAt,
	)

	// Assert invariant: All X.509 components are non-nil
	assert.NotNil(t, doc.Certificate(),
		"Invariant violated: Certificate() returned nil for X.509 document")
	assert.NotNil(t, doc.PrivateKey(),
		"Invariant violated: PrivateKey() returned nil for X.509 document")
	assert.NotNil(t, doc.Chain(),
		"Invariant violated: Chain() returned nil for X.509 document")
	assert.NotEmpty(t, doc.Chain(),
		"Invariant violated: Chain() returned empty slice for X.509 document")

	// Verify exact components match construction parameters
	assert.Equal(t, mockCert, doc.Certificate())
	assert.Equal(t, mockPrivateKey, doc.PrivateKey())
	assert.Equal(t, mockChain, doc.Chain())

	// Verify certificate contains SPIFFE ID in URI SAN (more authentic)
	assert.NotEmpty(t, doc.Certificate().URIs,
		"Certificate should contain URI SAN for SPIFFE ID")
	if len(doc.Certificate().URIs) > 0 {
		assert.Equal(t, "spiffe", doc.Certificate().URIs[0].Scheme,
			"URI SAN should use spiffe:// scheme")
		assert.Contains(t, doc.Certificate().URIs[0].String(), "example.org",
			"URI SAN should contain trust domain")
	}
}

// Helper functions for generating mock X.509 components for testing

// generateMockPrivateKey creates a mock ECDSA private key for testing.
// Returns *ecdsa.PrivateKey which implements crypto.PrivateKey interface.
func generateMockPrivateKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "Failed to generate mock private key")
	return privateKey
}

// generateMockCertificate creates a mock X.509 certificate for testing.
// The certificate includes a SPIFFE ID (spiffe://example.org/workload) in the URI SAN extension,
// making it more authentic to real SPIRE-issued X.509-SVIDs.
// The private key used is crypto.PrivateKey (*ecdsa.PrivateKey).
func generateMockCertificate(t *testing.T) *x509.Certificate {
	t.Helper()

	// Create a self-signed certificate for testing with SPIFFE ID
	privateKey := generateMockPrivateKey(t) // crypto.PrivateKey (*ecdsa.PrivateKey)

	// Create SPIFFE ID URI for Subject Alternative Name
	spiffeID, err := url.Parse("spiffe://example.org/workload")
	require.NoError(t, err, "Failed to parse SPIFFE ID")

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-workload",
		},
		NotBefore:             time.Unix(1000000000, 0), // January 9, 2001
		NotAfter:              time.Unix(2000000000, 0), // May 18, 2033
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		URIs:                  []*url.URL{spiffeID}, // SPIFFE ID as URI SAN
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err, "Failed to create mock certificate")

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err, "Failed to parse mock certificate")

	return cert
}
