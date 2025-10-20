//go:build dev

package inmemory_test

// Trust Bundle Provider Coverage Tests
//
// These tests verify edge cases and defensive improvements for the InMemory trust bundle provider.
// Tests cover nil handling, empty providers, mixed cert slices, and PEM encoding.
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/inmemory/... -v -run TestTrustBundleProvider_Coverage
//	go test ./internal/adapters/outbound/inmemory/... -cover

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/domain"
)

// TestTrustBundleProvider_Coverage_GetBundleNilTrustDomain tests nil trust domain rejection
func TestTrustBundleProvider_Coverage_GetBundleNilTrustDomain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange - Create provider with valid CA
	caCert := createTestCA(t, "Test CA")
	provider := inmemory.NewInMemoryTrustBundleProvider([]*x509.Certificate{caCert})

	// Act - Call GetBundle with nil trust domain
	bundle, err := provider.GetBundle(ctx, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "trust domain cannot be nil")
}

// TestTrustBundleProvider_Coverage_EmptyProvider tests empty provider error
func TestTrustBundleProvider_Coverage_EmptyProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	td := domain.NewTrustDomainFromName("example.org")

	tests := []struct {
		name    string
		caCerts []*x509.Certificate
		wantErr string
	}{
		{"nil slice", nil, "not found"},
		{"empty slice", []*x509.Certificate{}, "not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			provider := inmemory.NewInMemoryTrustBundleProvider(tt.caCerts)

			// Act
			bundle, err := provider.GetBundle(ctx, td)

			// Assert
			assert.Error(t, err)
			assert.Nil(t, bundle)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestTrustBundleProvider_Coverage_MixedSliceFiltering tests filtering of nil and non-CA certs
func TestTrustBundleProvider_Coverage_MixedSliceFiltering(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	td := domain.NewTrustDomainFromName("example.org")

	tests := []struct {
		name        string
		caCerts     []*x509.Certificate
		wantCACount int
		wantErr     bool
	}{
		{
			name:        "all nil entries",
			caCerts:     []*x509.Certificate{nil, nil, nil},
			wantCACount: 0,
			wantErr:     true, // No usable certs
		},
		{
			name: "mixed nil and valid CA",
			caCerts: []*x509.Certificate{
				nil,
				createTestCA(t, "CA1"),
				nil,
				createTestCA(t, "CA2"),
			},
			wantCACount: 2,
			wantErr:     false,
		},
		{
			name: "mixed CA and non-CA (leaf cert)",
			caCerts: []*x509.Certificate{
				createTestCA(t, "CA1"),
				createTestLeafCert(t, "leaf"), // Non-CA cert (should be filtered)
				createTestCA(t, "CA2"),
			},
			wantCACount: 2, // Only CAs should be included
			wantErr:     false,
		},
		{
			name: "all non-CA certs",
			caCerts: []*x509.Certificate{
				createTestLeafCert(t, "leaf1"),
				createTestLeafCert(t, "leaf2"),
			},
			wantCACount: 0,
			wantErr:     true, // No usable certs
		},
		{
			name: "mixed nil, CA, and non-CA",
			caCerts: []*x509.Certificate{
				nil,
				createTestCA(t, "CA1"),
				createTestLeafCert(t, "leaf"),
				nil,
				createTestCA(t, "CA2"),
				createTestLeafCert(t, "leaf2"),
			},
			wantCACount: 2, // Only the 2 CAs
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			provider := inmemory.NewInMemoryTrustBundleProvider(tt.caCerts)

			// Act
			bundle, err := provider.GetBundle(ctx, td)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, bundle)
				assert.Contains(t, err.Error(), "not found")
			} else {
				require.NoError(t, err)
				assert.NotNil(t, bundle)

				// Parse the PEM bundle to verify CA count
				certs := parsePEMBundle(t, bundle)
				assert.Equal(t, tt.wantCACount, len(certs), "should only contain CA certs")

				// Verify all returned certs are CAs
				for i, cert := range certs {
					assert.True(t, cert.IsCA, "cert %d should be a CA", i)
				}
			}
		})
	}
}

// TestTrustBundleProvider_Coverage_PEMEncoding tests PEM concatenation and parsing
func TestTrustBundleProvider_Coverage_PEMEncoding(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	td := domain.NewTrustDomainFromName("example.org")

	tests := []struct {
		name    string
		caCount int
	}{
		{"single CA", 1},
		{"two CAs", 2},
		{"five CAs", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange - Create multiple CA certs
			caCerts := make([]*x509.Certificate, tt.caCount)
			for i := 0; i < tt.caCount; i++ {
				caCerts[i] = createTestCA(t, "CA"+string(rune('A'+i)))
			}
			provider := inmemory.NewInMemoryTrustBundleProvider(caCerts)

			// Act
			bundle, err := provider.GetBundle(ctx, td)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, bundle)

			// Parse PEM bundle back into certs
			parsedCerts := parsePEMBundle(t, bundle)
			assert.Equal(t, tt.caCount, len(parsedCerts), "should parse back to same number of certs")

			// Verify each cert matches (by comparing Raw bytes)
			for i, parsed := range parsedCerts {
				assert.Equal(t, caCerts[i].Raw, parsed.Raw,
					"cert %d should match original", i)
			}
		})
	}
}

// TestTrustBundleProvider_Coverage_DefensiveCopy tests that provider stores defensive copy
func TestTrustBundleProvider_Coverage_DefensiveCopy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	td := domain.NewTrustDomainFromName("example.org")

	// Arrange - Create provider with CA
	originalCerts := []*x509.Certificate{createTestCA(t, "CA1")}
	provider := inmemory.NewInMemoryTrustBundleProvider(originalCerts)

	// Get initial bundle
	bundle1, err := provider.GetBundle(ctx, td)
	require.NoError(t, err)

	// Act - Mutate the original slice (should not affect provider)
	originalCerts[0] = createTestCA(t, "Different CA")
	originalCerts = append(originalCerts, createTestCA(t, "Extra CA"))

	// Assert - Get bundle again and verify it's unchanged
	bundle2, err := provider.GetBundle(ctx, td)
	require.NoError(t, err)

	// Bundles should be identical (provider has defensive copy)
	assert.Equal(t, bundle1, bundle2, "provider should store defensive copy")

	// Should only have 1 cert (not affected by append)
	parsedCerts := parsePEMBundle(t, bundle2)
	assert.Equal(t, 1, len(parsedCerts), "should still have original cert count")
}

// TestTrustBundleProvider_Coverage_GetBundleForIdentityNil tests nil identity rejection
func TestTrustBundleProvider_Coverage_GetBundleForIdentityNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Arrange - Create provider with valid CA
	caCert := createTestCA(t, "Test CA")
	provider := inmemory.NewInMemoryTrustBundleProvider([]*x509.Certificate{caCert})

	// Act - Call GetBundleForIdentity with nil credential
	bundle, err := provider.GetBundleForIdentity(ctx, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "identity credential cannot be nil")
}

// Helper: createTestCA creates a test CA certificate
func createTestCA(t *testing.T, cn string) *x509.Certificate {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	return cert
}

// Helper: createTestLeafCert creates a test leaf (non-CA) certificate
func createTestLeafCert(t *testing.T, cn string) *x509.Certificate {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false, // Not a CA
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	return cert
}

// Helper: parsePEMBundle parses a PEM bundle into certificates
func parsePEMBundle(t *testing.T, bundlePEM []byte) []*x509.Certificate {
	t.Helper()

	var certs []*x509.Certificate
	remaining := bundlePEM

	for len(remaining) > 0 {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			t.Fatalf("unexpected PEM block type: %s", block.Type)
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)

		certs = append(certs, cert)
		remaining = rest
	}

	return certs
}
