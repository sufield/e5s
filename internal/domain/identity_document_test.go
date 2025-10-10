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

// TestNewIdentityDocumentFromComponents_Success tests successful creation
// of an identity document with all X.509 components
func TestNewIdentityDocumentFromComponents_Success(t *testing.T) {
	t.Parallel()

	// Arrange - Use fixed time for determinism
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
	expiresAt := time.Unix(2000000000, 0) // May 18, 2033 - fixed future time

	// Create X.509 components
	cert := generateTestCertificate(t, "example.org", "/workload")
	privateKey := generateTestPrivateKey(t)
	chain := []*x509.Certificate{cert}

	// Act
	doc := domain.NewIdentityDocumentFromComponents(
		credential,
		cert,
		privateKey,
		chain,
		expiresAt,
	)

	// Assert - Verify all components are properly set
	require.NotNil(t, doc)

	// Test identity credential
	assert.NotNil(t, doc.IdentityCredential())
	assert.Equal(t, credential, doc.IdentityCredential())
	assert.Equal(t, "spiffe://example.org/workload", doc.IdentityCredential().String())

	// Test X.509 components are non-nil
	assert.NotNil(t, doc.Certificate(), "Certificate should be non-nil for X.509 document")
	assert.NotNil(t, doc.PrivateKey(), "PrivateKey should be non-nil for X.509 document")
	assert.NotNil(t, doc.Chain(), "Chain should be non-nil for X.509 document")
	assert.NotEmpty(t, doc.Chain(), "Chain should not be empty for X.509 document")

	// Test expiration
	assert.Equal(t, expiresAt, doc.ExpiresAt())
	assert.False(t, doc.IsExpired(), "Document should not be expired with future date")
	assert.True(t, doc.IsValid(), "Document should be valid when not expired")

	// Verify certificate contains SPIFFE ID in URI SAN
	assert.NotEmpty(t, doc.Certificate().URIs, "Certificate should contain URI SAN")
	if len(doc.Certificate().URIs) > 0 {
		assert.Equal(t, "spiffe", doc.Certificate().URIs[0].Scheme)
		assert.Contains(t, doc.Certificate().URIs[0].String(), "example.org")
	}
}

// TestNewIdentityDocumentFromComponents_MinimalValid tests creation
// with minimal valid configuration (no X.509 components)
func TestNewIdentityDocumentFromComponents_MinimalValid(t *testing.T) {
	t.Parallel()

	// Arrange
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
	expiresAt := time.Unix(2000000000, 0) // May 18, 2033

	// Act - Create document without X.509 components
	doc := domain.NewIdentityDocumentFromComponents(
		credential,
		nil, // no cert
		nil, // no privateKey
		nil, // no chain
		expiresAt,
	)

	// Assert - Verify identity credential is always non-nil
	require.NotNil(t, doc)
	assert.NotNil(t, doc.IdentityCredential(), "IdentityCredential should always be non-nil")
	assert.Equal(t, credential, doc.IdentityCredential())

	// X.509 components can be nil for non-X.509 documents
	assert.Nil(t, doc.Certificate())
	assert.Nil(t, doc.PrivateKey())
	assert.Nil(t, doc.Chain())

	// Expiration still works
	assert.Equal(t, expiresAt, doc.ExpiresAt())
	assert.False(t, doc.IsExpired())
}

// TestIdentityDocument_ExpirationBehavior tests expiration logic
func TestIdentityDocument_ExpirationBehavior(t *testing.T) {
	t.Parallel()

	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")

	tests := []struct {
		name            string
		expiresAt       time.Time
		expectedExpired bool
		expectedValid   bool
	}{
		{
			name:            "future expiration",
			expiresAt:       time.Unix(2000000000, 0), // May 18, 2033
			expectedExpired: false,
			expectedValid:   true,
		},
		{
			name:            "past expiration",
			expiresAt:       time.Unix(1000000000, 0), // January 9, 2001
			expectedExpired: true,
			expectedValid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			doc := domain.NewIdentityDocumentFromComponents(
				credential,
				nil, nil, nil,
				tt.expiresAt,
			)

			// Assert
			assert.Equal(t, tt.expectedExpired, doc.IsExpired())
			assert.Equal(t, tt.expectedValid, doc.IsValid())
			assert.Equal(t, !doc.IsExpired(), doc.IsValid(), "IsValid() should equal !IsExpired()")
		})
	}
}

// Helper functions for generating test X.509 components

// generateTestPrivateKey creates a test ECDSA private key
// Returns *ecdsa.PrivateKey which implements crypto.PrivateKey interface
func generateTestPrivateKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "Failed to generate test private key")
	return privateKey
}

// generateTestCertificate creates a test X.509 certificate with SPIFFE ID
// The certificate includes a SPIFFE ID in the URI SAN extension,
// matching real SPIRE-issued X.509-SVIDs
func generateTestCertificate(t *testing.T, trustDomain, path string) *x509.Certificate {
	t.Helper()

	// Create a self-signed certificate with SPIFFE ID
	privateKey := generateTestPrivateKey(t)

	// Create SPIFFE ID URI for Subject Alternative Name
	spiffeID, err := url.Parse("spiffe://" + trustDomain + path)
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
	require.NoError(t, err, "Failed to create test certificate")

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err, "Failed to parse test certificate")

	return cert
}
