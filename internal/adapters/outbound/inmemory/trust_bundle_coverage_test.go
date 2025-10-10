package inmemory_test

// TrustBundle Coverage Tests
//
// These tests verify edge cases and error paths for the inmemory TrustBundleProvider implementation.
// Tests cover bundle retrieval, nil handling, empty CAs, and multi-CA concatenation.
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/inmemory/... -v -run TestTrustBundle
//	go test ./internal/adapters/outbound/inmemory/... -cover

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"regexp"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrustBundleProvider_GetBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	// Create server with CA
	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	// Create trust bundle provider with server's CA
	caCerts := []*x509.Certificate{server.GetCA()}
	provider := inmemory.NewInMemoryTrustBundleProvider(caCerts)

	// Act - Get bundle for trust domain
	td := domain.NewTrustDomainFromName("example.org")
	bundle, err := provider.GetBundle(ctx, td)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, bundle)
	assert.NotEmpty(t, bundle)
	assert.Contains(t, string(bundle), "BEGIN CERTIFICATE")
}

// TestTrustBundleProvider_GetBundleForIdentity tests bundle for specific identity
func TestTrustBundleProvider_GetBundleForIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	caCerts := []*x509.Certificate{server.GetCA()}
	provider := inmemory.NewInMemoryTrustBundleProvider(caCerts)

	// Create identity credential
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")

	// Act
	bundle, err := provider.GetBundleForIdentity(ctx, credential)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, bundle)
	assert.Contains(t, string(bundle), "BEGIN CERTIFICATE")
}

// TestTrustBundleProvider_EmptyCAs tests error case with no CAs
func TestTrustBundleProvider_EmptyCAs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create provider with empty CA list
	provider := inmemory.NewInMemoryTrustBundleProvider(nil)

	td := domain.NewTrustDomainFromName("example.org")

	// Act
	bundle, err := provider.GetBundle(ctx, td)

	// Assert - Should return error for empty CAs
	assert.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "trust bundle not found")
}

// TestTrustBundleProvider_GetBundle_NilTrustDomain tests nil trust domain error
func TestTrustBundleProvider_GetBundle_NilTrustDomain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	caCerts := []*x509.Certificate{server.GetCA()}
	provider := inmemory.NewInMemoryTrustBundleProvider(caCerts)

	// Act - Pass nil trust domain
	bundle, err := provider.GetBundle(ctx, nil)

	// Assert - Should return error
	assert.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "trust domain cannot be nil")
}
func TestTrustBundleProvider_GetBundleForIdentity_NilNamespace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	caCerts := []*x509.Certificate{server.GetCA()}
	provider := inmemory.NewInMemoryTrustBundleProvider(caCerts)

	// Act - Pass nil identity credential
	bundle, err := provider.GetBundleForIdentity(ctx, nil)

	// Assert - Should return error
	assert.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "identity credential cannot be nil")
}
func TestTrustBundleProvider_MultiCAConcat(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	// Create two separate servers to get two different CAs
	server1, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	server2, err := inmemory.NewInMemoryServer(ctx, "example.com", tdParser, docProvider)
	require.NoError(t, err)

	// Create provider with multiple CAs
	caCerts := []*x509.Certificate{server1.GetCA(), server2.GetCA()}
	provider := inmemory.NewInMemoryTrustBundleProvider(caCerts)

	td := domain.NewTrustDomainFromName("example.org")

	// Act
	bundle, err := provider.GetBundle(ctx, td)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, bundle)

	// Verify it contains multiple PEM blocks
	bundleStr := string(bundle)
	assert.Contains(t, bundleStr, "BEGIN CERTIFICATE")
	assert.Contains(t, bundleStr, "END CERTIFICATE")

	// Count PEM blocks (should be 2)
	beginCount := len(regexp.MustCompile("BEGIN CERTIFICATE").FindAllString(bundleStr, -1))
	endCount := len(regexp.MustCompile("END CERTIFICATE").FindAllString(bundleStr, -1))
	assert.Equal(t, 2, beginCount, "Should have 2 BEGIN CERTIFICATE markers")
	assert.Equal(t, 2, endCount, "Should have 2 END CERTIFICATE markers")

	// Verify the bundle can be parsed back
	block1, rest := pem.Decode(bundle)
	assert.NotNil(t, block1, "First PEM block should decode")
	assert.Equal(t, "CERTIFICATE", block1.Type)

	block2, _ := pem.Decode(rest)
	assert.NotNil(t, block2, "Second PEM block should decode")
	assert.Equal(t, "CERTIFICATE", block2.Type)
}
