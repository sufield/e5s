package spire

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// mockBundleSource implements x509bundle.Source for testing
type mockBundleSource struct {
	bundles map[spiffeid.TrustDomain]*x509bundle.Bundle
}

func (m *mockBundleSource) GetX509BundleForTrustDomain(td spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	bundle, ok := m.bundles[td]
	if !ok {
		return nil, domain.ErrTrustBundleNotFound
	}
	return bundle, nil
}

// Helper: Create a test CA
func createTestCA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create CA certificate: %v", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}

	return caCert, caKey
}

// Helper: Create a test SVID
func createTestSVID(t *testing.T, caCert *x509.Certificate, caKey *rsa.PrivateKey, spiffeIDStr string) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()

	svidKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate SVID key: %v", err)
	}

	spiffeURI, err := url.Parse(spiffeIDStr)
	if err != nil {
		t.Fatalf("invalid SPIFFE ID: %v", err)
	}

	svidTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: spiffeIDStr},
		URIs:         []*url.URL{spiffeURI},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	svidCertDER, err := x509.CreateCertificate(rand.Reader, svidTemplate, caCert, &svidKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create SVID certificate: %v", err)
	}

	svidCert, err := x509.ParseCertificate(svidCertDER)
	if err != nil {
		t.Fatalf("failed to parse SVID certificate: %v", err)
	}

	return svidCert, svidKey
}

func TestSDKDocumentProvider_CreateX509IdentityDocument_ReturnsError(t *testing.T) {
	// In production, certificate creation is delegated to SPIRE Server
	provider := &SDKDocumentProvider{bundleSource: nil}

	ctx := context.Background()
	trustDomain := domain.NewTrustDomainFromName("example.org")
	identityCredential := domain.NewIdentityCredentialFromComponents(trustDomain, "/test")

	_, err := provider.CreateX509IdentityDocument(ctx, identityCredential, nil, nil)
	if err == nil {
		t.Fatal("expected error for certificate creation, got nil")
	}

	// Should return domain sentinel error
	if !domain.IsIdentityDocumentInvalid(err) {
		t.Errorf("expected ErrIdentityDocumentInvalid, got: %v", err)
	}
}

func TestSDKDocumentProvider_ValidateIdentityDocument_Success(t *testing.T) {
	// Setup: Create test CA and SVID
	caCert, caKey := createTestCA(t)
	spiffeIDStr := "spiffe://example.org/test"
	svidCert, svidKey := createTestSVID(t, caCert, caKey, spiffeIDStr)

	// Create bundle source with CA
	td := spiffeid.RequireTrustDomainFromString("example.org")
	bundle := x509bundle.FromX509Authorities(td, []*x509.Certificate{caCert})
	bundleSource := &mockBundleSource{
		bundles: map[spiffeid.TrustDomain]*x509bundle.Bundle{
			td: bundle,
		},
	}

	// Create provider
	provider := NewSDKDocumentProvider(bundleSource)

	// Create domain identity document
	trustDomain := domain.NewTrustDomainFromName("example.org")
	identityCredential := domain.NewIdentityCredentialFromComponents(trustDomain, "/test")
	doc := domain.NewIdentityDocumentFromComponents(
		identityCredential,
		svidCert,
		svidKey,
		[]*x509.Certificate{svidCert, caCert},
		svidCert.NotAfter,
	)

	// Test: Validate should succeed
	ctx := context.Background()
	err := provider.ValidateIdentityDocument(ctx, doc, identityCredential)
	if err != nil {
		t.Errorf("expected validation to succeed, got error: %v", err)
	}
}

func TestSDKDocumentProvider_ValidateIdentityDocument_NilDocument(t *testing.T) {
	provider := &SDKDocumentProvider{bundleSource: nil}

	ctx := context.Background()
	trustDomain := domain.NewTrustDomainFromName("example.org")
	identityCredential := domain.NewIdentityCredentialFromComponents(trustDomain, "/test")

	err := provider.ValidateIdentityDocument(ctx, nil, identityCredential)
	if err == nil {
		t.Fatal("expected error for nil document")
	}

	if !domain.IsIdentityDocumentInvalid(err) {
		t.Errorf("expected ErrIdentityDocumentInvalid, got: %v", err)
	}
}

func TestSDKDocumentProvider_ValidateIdentityDocument_NilExpectedID(t *testing.T) {
	provider := &SDKDocumentProvider{bundleSource: nil}

	ctx := context.Background()
	trustDomain := domain.NewTrustDomainFromName("example.org")
	identityCredential := domain.NewIdentityCredentialFromComponents(trustDomain, "/test")

	// Create minimal doc (won't reach bundle check due to early validation)
	doc := domain.NewIdentityDocumentFromComponents(
		identityCredential,
		nil,
		nil,
		nil,
		time.Now().Add(1*time.Hour),
	)

	err := provider.ValidateIdentityDocument(ctx, doc, nil)
	if err == nil {
		t.Fatal("expected error for nil expected ID")
	}

	if !domain.IsInvalidIdentityCredential(err) {
		t.Errorf("expected ErrInvalidIdentityCredential, got: %v", err)
	}
}

func TestSDKDocumentProvider_ValidateIdentityDocument_Expired(t *testing.T) {
	// Use a provider with a custom clock for testing expiration
	provider := &SDKDocumentProvider{
		bundleSource: nil,
		clock:        time.Now,
		clockSkew:    5 * time.Minute,
	}

	ctx := context.Background()
	trustDomain := domain.NewTrustDomainFromName("example.org")
	identityCredential := domain.NewIdentityCredentialFromComponents(trustDomain, "/test")

	// Create test CA and expired SVID
	caCert, caKey := createTestCA(t)
	svidKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate SVID key: %v", err)
	}

	spiffeURI, _ := url.Parse("spiffe://example.org/test")
	expiredNotAfter := time.Now().Add(-10 * time.Minute) // Expired 10 minutes ago (beyond clock skew)
	svidTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "spiffe://example.org/test"},
		URIs:         []*url.URL{spiffeURI},
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     expiredNotAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	svidCertDER, err := x509.CreateCertificate(rand.Reader, svidTemplate, caCert, &svidKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create expired SVID: %v", err)
	}

	svidCert, err := x509.ParseCertificate(svidCertDER)
	if err != nil {
		t.Fatalf("failed to parse expired SVID: %v", err)
	}

	// Create expired document
	doc := domain.NewIdentityDocumentFromComponents(
		identityCredential,
		svidCert,
		svidKey,
		[]*x509.Certificate{svidCert},
		svidCert.NotAfter,
	)

	err = provider.ValidateIdentityDocument(ctx, doc, identityCredential)
	if err == nil {
		t.Fatal("expected error for expired document")
	}

	if !errors.Is(err, domain.ErrIdentityDocumentExpired) {
		t.Errorf("expected ErrIdentityDocumentExpired, got: %v", err)
	}
}

func TestSDKDocumentProvider_ValidateIdentityDocument_IdentityMismatch(t *testing.T) {
	provider := &SDKDocumentProvider{
		bundleSource: nil,
		clock:        time.Now,
		clockSkew:    5 * time.Minute,
	}

	ctx := context.Background()
	trustDomain := domain.NewTrustDomainFromName("example.org")
	actualID := domain.NewIdentityCredentialFromComponents(trustDomain, "/actual")
	expectedID := domain.NewIdentityCredentialFromComponents(trustDomain, "/expected")

	// Create test CA and SVID with actualID
	caCert, caKey := createTestCA(t)
	spiffeIDStr := "spiffe://example.org/actual"
	svidCert, svidKey := createTestSVID(t, caCert, caKey, spiffeIDStr)

	// Create document with actualID (different from expectedID)
	doc := domain.NewIdentityDocumentFromComponents(
		actualID, // Different from expected
		svidCert,
		svidKey,
		[]*x509.Certificate{svidCert},
		svidCert.NotAfter,
	)

	err := provider.ValidateIdentityDocument(ctx, doc, expectedID)
	if err == nil {
		t.Fatal("expected error for identity mismatch")
	}

	if !domain.IsIdentityDocumentMismatch(err) {
		t.Errorf("expected ErrIdentityDocumentMismatch, got: %v", err)
	}
}

func TestSDKDocumentProvider_ValidateIdentityDocument_BundleNotFound(t *testing.T) {
	// Setup: Bundle source without the required trust domain
	bundleSource := &mockBundleSource{
		bundles: map[spiffeid.TrustDomain]*x509bundle.Bundle{},
	}

	provider := NewSDKDocumentProvider(bundleSource)

	// Create test CA and SVID
	caCert, caKey := createTestCA(t)
	spiffeIDStr := "spiffe://example.org/test"
	svidCert, svidKey := createTestSVID(t, caCert, caKey, spiffeIDStr)

	// Create domain document
	trustDomain := domain.NewTrustDomainFromName("example.org")
	identityCredential := domain.NewIdentityCredentialFromComponents(trustDomain, "/test")
	doc := domain.NewIdentityDocumentFromComponents(
		identityCredential,
		svidCert,
		svidKey,
		[]*x509.Certificate{svidCert},
		svidCert.NotAfter,
	)

	// Test: Should fail with bundle not found
	ctx := context.Background()
	err := provider.ValidateIdentityDocument(ctx, doc, identityCredential)
	if err == nil {
		t.Fatal("expected error for missing bundle")
	}

	if !domain.IsCertificateChainInvalid(err) {
		t.Errorf("expected ErrCertificateChainInvalid, got: %v", err)
	}
}

func TestSDKDocumentProvider_ValidateIdentityDocument_EmptyChain(t *testing.T) {
	provider := &SDKDocumentProvider{bundleSource: nil}

	ctx := context.Background()
	trustDomain := domain.NewTrustDomainFromName("example.org")
	identityCredential := domain.NewIdentityCredentialFromComponents(trustDomain, "/test")

	// Create document with empty chain
	doc := domain.NewIdentityDocumentFromComponents(
		identityCredential,
		nil,
		nil,
		[]*x509.Certificate{}, // Empty chain
		time.Now().Add(1*time.Hour),
	)

	err := provider.ValidateIdentityDocument(ctx, doc, identityCredential)
	if err == nil {
		t.Fatal("expected error for empty certificate chain")
	}

	if !domain.IsIdentityDocumentInvalid(err) {
		t.Errorf("expected ErrIdentityDocumentInvalid, got: %v", err)
	}
}

func TestSDKDocumentProvider_ValidateIdentityDocument_WrongTrustDomain(t *testing.T) {
	// Setup: Bundle source with only one trust domain
	td1 := spiffeid.RequireTrustDomainFromString("example.org")
	caCert, caKey := createTestCA(t)
	bundle := x509bundle.FromX509Authorities(td1, []*x509.Certificate{caCert})
	bundleSource := &mockBundleSource{
		bundles: map[spiffeid.TrustDomain]*x509bundle.Bundle{
			td1: bundle,
		},
	}

	provider := NewSDKDocumentProvider(bundleSource)

	// Create document for different trust domain
	trustDomain := domain.NewTrustDomainFromName("other.org")
	identityCredential := domain.NewIdentityCredentialFromComponents(trustDomain, "/test")

	// Create minimal chain (will fail bundle lookup for other.org)
	spiffeIDStr := "spiffe://other.org/test"
	svidCert, svidKey := createTestSVID(t, caCert, caKey, spiffeIDStr)

	doc := domain.NewIdentityDocumentFromComponents(
		identityCredential,
		svidCert,
		svidKey,
		[]*x509.Certificate{svidCert},
		svidCert.NotAfter,
	)

	// Test: Should fail with bundle not found for wrong trust domain
	ctx := context.Background()
	err := provider.ValidateIdentityDocument(ctx, doc, identityCredential)
	if err == nil {
		t.Fatal("expected error for wrong trust domain (no bundle)")
	}

	if !domain.IsCertificateChainInvalid(err) {
		t.Errorf("expected ErrCertificateChainInvalid, got: %v", err)
	}
}

func TestSDKDocumentProvider_ValidateIdentityDocument_WrongSPIFFEID(t *testing.T) {
	// Setup: Create CA and SVID
	caCert, caKey := createTestCA(t)
	spiffeID1 := "spiffe://example.org/workload1"
	svidCert1, svidKey1 := createTestSVID(t, caCert, caKey, spiffeID1)

	// Create bundle
	td := spiffeid.RequireTrustDomainFromString("example.org")
	bundle := x509bundle.FromX509Authorities(td, []*x509.Certificate{caCert})
	bundleSource := &mockBundleSource{
		bundles: map[spiffeid.TrustDomain]*x509bundle.Bundle{
			td: bundle,
		},
	}

	provider := NewSDKDocumentProvider(bundleSource)

	// Create document with spiffeID1
	trustDomain := domain.NewTrustDomainFromName("example.org")
	actualID := domain.NewIdentityCredentialFromComponents(trustDomain, "/workload1")
	expectedID := domain.NewIdentityCredentialFromComponents(trustDomain, "/workload2")

	doc := domain.NewIdentityDocumentFromComponents(
		actualID,
		svidCert1,
		svidKey1,
		[]*x509.Certificate{svidCert1, caCert},
		svidCert1.NotAfter,
	)

	// Test: Validation should fail before SDK verification (domain-level mismatch)
	ctx := context.Background()
	err := provider.ValidateIdentityDocument(ctx, doc, expectedID)
	if err == nil {
		t.Fatal("expected error for SPIFFE ID mismatch")
	}

	if !domain.IsIdentityDocumentMismatch(err) {
		t.Errorf("expected ErrIdentityDocumentMismatch, got: %v", err)
	}
}

func TestSDKDocumentProvider_InterfaceCompliance(t *testing.T) {
	// Compile-time check
	var _ = NewSDKDocumentProvider(nil)
}
