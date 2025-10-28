package spire

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sufield/e5s/internal/domain"
)

func TestTranslateTrustDomainToSPIFFEID(t *testing.T) {
	tests := []struct {
		name        string
		trustDomain *domain.TrustDomain
		want        string
		wantErr     bool
	}{
		{
			name:        "valid trust domain",
			trustDomain: domain.NewTrustDomainFromName("example.org"),
			want:        "example.org",
			wantErr:     false,
		},
		{
			name:        "another valid domain",
			trustDomain: domain.NewTrustDomainFromName("test.com"),
			want:        "test.com",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TranslateTrustDomainToSPIFFEID(tt.trustDomain)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got.String())
			}
		})
	}
}

func TestTranslateIdentityCredentialToSPIFFEID(t *testing.T) {
	tests := []struct {
		name       string
		credential *domain.IdentityCredential
		want       string
		wantErr    bool
	}{
		{
			name: "with path",
			credential: domain.NewIdentityCredentialFromComponents(
				domain.NewTrustDomainFromName("example.org"),
				"/workload/server",
			),
			want:    "spiffe://example.org/workload/server",
			wantErr: false,
		},
		{
			name: "single segment path",
			credential: domain.NewIdentityCredentialFromComponents(
				domain.NewTrustDomainFromName("example.org"),
				"/workload",
			),
			want:    "spiffe://example.org/workload",
			wantErr: false,
		},
		{
			name: "root ID (path is /)",
			credential: domain.NewIdentityCredentialFromComponents(
				domain.NewTrustDomainFromName("example.org"),
				"/",
			),
			want:    "spiffe://example.org",
			wantErr: false,
		},
		{
			name: "relative path without leading slash",
			credential: domain.NewIdentityCredentialFromComponents(
				domain.NewTrustDomainFromName("example.org"),
				"workload/server",
			),
			want:    "spiffe://example.org/workload/server",
			wantErr: false,
		},
		{
			name: "deep path",
			credential: domain.NewIdentityCredentialFromComponents(
				domain.NewTrustDomainFromName("example.org"),
				"/ns/production/workload/api/v1",
			),
			want:    "spiffe://example.org/ns/production/workload/api/v1",
			wantErr: false,
		},
		{
			name:       "nil credential",
			credential: nil,
			want:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TranslateIdentityCredentialToSPIFFEID(tt.credential)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, domain.ErrInvalidIdentityCredential)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got.String())
				// Verify the result is a valid SPIFFE ID (no invalid URIs)
				assert.False(t, got.IsZero(), "Result should be a valid SPIFFE ID")
			}
		})
	}
}

func TestTranslateSPIFFEIDToIdentityCredential(t *testing.T) {
	tests := []struct {
		name       string
		spiffeID   string
		wantDomain string
		wantPath   string
		wantErr    bool
	}{
		{
			name:       "valid SPIFFE ID with path",
			spiffeID:   "spiffe://example.org/workload/server",
			wantDomain: "example.org",
			wantPath:   "/workload/server",
			wantErr:    false,
		},
		{
			name:       "complex path",
			spiffeID:   "spiffe://trust.domain/ns/workload/v1",
			wantDomain: "trust.domain",
			wantPath:   "/ns/workload/v1",
			wantErr:    false,
		},
		{
			name:       "root ID (no path)",
			spiffeID:   "spiffe://example.org",
			wantDomain: "example.org",
			wantPath:   "/", // Domain model uses "/" to denote root
			wantErr:    false,
		},
		{
			name:       "single segment path",
			spiffeID:   "spiffe://example.org/workload",
			wantDomain: "example.org",
			wantPath:   "/workload",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := spiffeid.FromString(tt.spiffeID)
			require.NoError(t, err, "Failed to parse test SPIFFE ID")

			got, err := TranslateSPIFFEIDToIdentityCredential(id)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantDomain, got.TrustDomain().String())
				assert.Equal(t, tt.wantPath, got.Path())
			}
		})
	}
}

// TestTranslateSPIFFEIDToIdentityCredential_ZeroID tests that zero-value SPIFFE IDs are rejected
func TestTranslateSPIFFEIDToIdentityCredential_ZeroID(t *testing.T) {
	var zeroID spiffeid.ID // Zero value
	got, err := TranslateSPIFFEIDToIdentityCredential(zeroID)
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, domain.ErrInvalidIdentityCredential)
}

func TestTranslateX509SVIDToIdentityDocument(t *testing.T) {
	// Create a test certificate
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certBytes)
	require.NoError(t, err)

	// Create SPIFFE ID
	spiffeID, err := spiffeid.FromString("spiffe://example.org/workload")
	require.NoError(t, err)

	// Create X.509 SVID
	svid := &x509svid.SVID{
		ID:           spiffeID,
		Certificates: []*x509.Certificate{cert},
		PrivateKey:   privateKey,
	}

	// Translate to identity document
	doc, err := TranslateX509SVIDToIdentityDocument(svid)
	require.NoError(t, err)

	assert.NotNil(t, doc)
	assert.Equal(t, "spiffe://example.org/workload", doc.IdentityCredential().String())
	assert.NotNil(t, doc.Certificate())
	// Note: PrivateKey is no longer stored in domain.IdentityDocument
	// Private keys are managed separately by adapters (in X509SVID or dto.Identity)
	assert.True(t, doc.IsValid())
	assert.False(t, doc.IsExpired())
}

func TestTranslateX509SVIDToIdentityDocument_NilSVID(t *testing.T) {
	doc, err := TranslateX509SVIDToIdentityDocument(nil)
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.ErrorIs(t, err, domain.ErrIdentityDocumentInvalid)
}

func TestTranslateX509SVIDToIdentityDocument_NoCertificates(t *testing.T) {
	spiffeID, err := spiffeid.FromString("spiffe://example.org/workload")
	require.NoError(t, err)

	svid := &x509svid.SVID{
		ID:           spiffeID,
		Certificates: []*x509.Certificate{}, // Empty certificates
		PrivateKey:   nil,
	}

	doc, err := TranslateX509SVIDToIdentityDocument(svid)
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.ErrorIs(t, err, domain.ErrIdentityDocumentInvalid)
	assert.Contains(t, err.Error(), "missing leaf certificate")
}

func TestTranslateX509SVIDToIdentityDocument_NilLeafCertificate(t *testing.T) {
	spiffeID, err := spiffeid.FromString("spiffe://example.org/workload")
	require.NoError(t, err)

	svid := &x509svid.SVID{
		ID:           spiffeID,
		Certificates: []*x509.Certificate{nil}, // Nil leaf certificate
		PrivateKey:   nil,
	}

	doc, err := TranslateX509SVIDToIdentityDocument(svid)
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.ErrorIs(t, err, domain.ErrIdentityDocumentInvalid)
	assert.Contains(t, err.Error(), "missing leaf certificate")
}

func TestTranslateX509SVIDToIdentityDocument_MissingPrivateKey(t *testing.T) {
	// Create a test certificate
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certBytes)
	require.NoError(t, err)

	spiffeID, err := spiffeid.FromString("spiffe://example.org/workload")
	require.NoError(t, err)

	svid := &x509svid.SVID{
		ID:           spiffeID,
		Certificates: []*x509.Certificate{cert},
		PrivateKey:   nil, // Missing private key
	}

	doc, err := TranslateX509SVIDToIdentityDocument(svid)
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.ErrorIs(t, err, domain.ErrIdentityDocumentInvalid)
	assert.Contains(t, err.Error(), "missing/invalid private key")
}

// TestTranslateX509SVIDToIdentityDocument_SliceIsolation verifies defensive copy
func TestTranslateX509SVIDToIdentityDocument_SliceIsolation(t *testing.T) {
	// Create a test certificate
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certBytes)
	require.NoError(t, err)

	spiffeID, err := spiffeid.FromString("spiffe://example.org/workload")
	require.NoError(t, err)

	// Create SVID with certificate chain
	originalCerts := []*x509.Certificate{cert}
	svid := &x509svid.SVID{
		ID:           spiffeID,
		Certificates: originalCerts,
		PrivateKey:   privateKey,
	}

	// Translate to identity document
	doc, err := TranslateX509SVIDToIdentityDocument(svid)
	require.NoError(t, err)

	// Modify the original slice
	originalCerts[0] = nil

	// Verify the document's certificate chain is not affected
	assert.NotNil(t, doc.Certificate(), "Document certificate should not be affected by original slice mutation")
	assert.Equal(t, cert, doc.Certificate(), "Document certificate should match original")
}
