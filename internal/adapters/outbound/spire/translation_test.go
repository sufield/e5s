package spire

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestTranslateIdentityNamespaceToSPIFFEID(t *testing.T) {
	tests := []struct {
		name      string
		namespace *domain.IdentityNamespace
		want      string
		wantErr   bool
	}{
		{
			name: "with path",
			namespace: domain.NewIdentityNamespaceFromComponents(
				domain.NewTrustDomainFromName("example.org"),
				"/workload/server",
			),
			want:    "spiffe://example.org/workload/server",
			wantErr: false,
		},
		{
			name: "single segment path",
			namespace: domain.NewIdentityNamespaceFromComponents(
				domain.NewTrustDomainFromName("example.org"),
				"/workload",
			),
			want:    "spiffe://example.org/workload",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TranslateIdentityNamespaceToSPIFFEID(tt.namespace)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got.String())
			}
		})
	}
}

func TestTranslateSPIFFEIDToIdentityNamespace(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := spiffeid.FromString(tt.spiffeID)
			require.NoError(t, err, "Failed to parse test SPIFFE ID")

			got, err := TranslateSPIFFEIDToIdentityNamespace(id)
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
	assert.Equal(t, "spiffe://example.org/workload", doc.IdentityNamespace().String())
	assert.Equal(t, domain.IdentityDocumentTypeX509, doc.Type())
	assert.NotNil(t, doc.Certificate())
	assert.NotNil(t, doc.PrivateKey())
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
}
