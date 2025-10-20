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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pocket/hexagon/spire/internal/domain"
)

// Note: generateTestPrivateKey() is kept for certificate generation helpers,
// even though private keys are no longer stored in domain.IdentityDocument

// TestNewIdentityDocumentFromComponents_Success tests successful creation
// of an identity document with all X.509 components
func TestNewIdentityDocumentFromComponents_Success(t *testing.T) {
	t.Parallel()

	// Arrange - Use fixed time for determinism
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")

	// Create X.509 components
	cert := generateTestCertificate(t, "example.org", "/workload")
	chain := []*x509.Certificate{cert}

	// Act
	doc, err := domain.NewIdentityDocumentFromComponents(
		credential,
		cert,
		chain,
	)

	// Assert - Verify all components are properly set
	require.NoError(t, err)
	require.NotNil(t, doc)

	// Test identity credential
	assert.NotNil(t, doc.IdentityCredential())
	assert.Equal(t, credential, doc.IdentityCredential())
	assert.Equal(t, "spiffe://example.org/workload", doc.IdentityCredential().String())

	// Test X.509 components are non-nil
	assert.NotNil(t, doc.Certificate(), "Certificate should be non-nil for X.509 document")
	assert.NotNil(t, doc.Chain(), "Chain should be non-nil for X.509 document")
	assert.NotEmpty(t, doc.Chain(), "Chain should not be empty for X.509 document")

	// Test expiration derived from certificate
	assert.Equal(t, cert.NotAfter, doc.ExpiresAt(), "ExpiresAt should match cert.NotAfter")
	assert.Equal(t, cert.NotBefore, doc.NotBefore(), "NotBefore should match cert.NotBefore")
	assert.False(t, doc.IsExpired(), "Document should not be expired with future date")
	assert.True(t, doc.IsValid(), "Document should be valid when not expired")

	// Verify certificate contains SPIFFE ID in URI SAN
	assert.NotEmpty(t, doc.Certificate().URIs, "Certificate should contain URI SAN")
	if len(doc.Certificate().URIs) > 0 {
		assert.Equal(t, "spiffe", doc.Certificate().URIs[0].Scheme)
		assert.Contains(t, doc.Certificate().URIs[0].String(), "example.org")
	}
}

// TestNewIdentityDocumentFromComponents_ValidationErrors tests validation errors
func TestNewIdentityDocumentFromComponents_ValidationErrors(t *testing.T) {
	t.Parallel()

	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
	cert := generateTestCertificate(t, "example.org", "/workload")
	chain := []*x509.Certificate{cert}

	tests := []struct {
		name       string
		credential *domain.IdentityCredential
		cert       *x509.Certificate
		chain      []*x509.Certificate
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "nil identity credential",
			credential: nil,
			cert:       cert,
			chain:      chain,
			wantErr:    true,
			errMsg:     "identity credential cannot be nil",
		},
		{
			name:       "nil certificate",
			credential: credential,
			cert:       nil,
			chain:      chain,
			wantErr:    true,
			errMsg:     "certificate cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			doc, err := domain.NewIdentityDocumentFromComponents(
				tt.credential,
				tt.cert,
				tt.chain,
			)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, doc)
				assert.ErrorIs(t, err, domain.ErrIdentityDocumentInvalid)
			} else {
				require.NoError(t, err)
				require.NotNil(t, doc)
			}
		})
	}
}

// TestIdentityDocument_ExpirationBehavior tests expiration logic
func TestIdentityDocument_ExpirationBehavior(t *testing.T) {
	t.Parallel()

	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")

	tests := []struct {
		name            string
		notBefore       time.Time
		notAfter        time.Time
		expectedExpired bool
		expectedValid   bool
	}{
		{
			name:            "future expiration",
			notBefore:       time.Unix(1000000000, 0), // January 9, 2001
			notAfter:        time.Unix(2000000000, 0), // May 18, 2033
			expectedExpired: false,
			expectedValid:   true,
		},
		{
			name:            "past expiration",
			notBefore:       time.Unix(500000000, 0),  // October 26, 1985
			notAfter:        time.Unix(1000000000, 0), // January 9, 2001
			expectedExpired: true,
			expectedValid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create certificate with specific expiration
			cert := generateTestCertificateWithTimes(t, "example.org", "/workload", tt.notBefore, tt.notAfter)

			// Act
			doc, err := domain.NewIdentityDocumentFromComponents(
				credential,
				cert,
				nil,
			)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.expectedExpired, doc.IsExpired())
			assert.Equal(t, tt.expectedValid, doc.IsValid())
			assert.Equal(t, !doc.IsExpired(), doc.IsValid(), "IsValid() should equal !IsExpired()")
		})
	}
}

// Helper functions for generating test X.509 components

// generateTestPrivateKey creates a test ECDSA private key
// Returns *ecdsa.PrivateKey which implements crypto.Signer interface
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
	return generateTestCertificateWithTimes(
		t,
		trustDomain,
		path,
		time.Unix(1000000000, 0), // January 9, 2001
		time.Unix(2000000000, 0), // May 18, 2033
	)
}

// generateTestCertificateWithTimes creates a test X.509 certificate with specific validity times
func generateTestCertificateWithTimes(t *testing.T, trustDomain, path string, notBefore, notAfter time.Time) *x509.Certificate {
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
		NotBefore:             notBefore,
		NotAfter:              notAfter,
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
