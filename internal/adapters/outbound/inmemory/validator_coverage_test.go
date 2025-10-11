package inmemory_test

// Validator Coverage Tests
//
// These tests verify edge cases and error paths for the inmemory IdentityDocumentValidator implementation.
// Tests cover nil handling, document expiration, credential matching, and successful validation.
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/inmemory/... -v -run TestIdentityDocumentValidator
//	go test ./internal/adapters/outbound/inmemory/... -cover

import (
	"context"
	"crypto/x509"
	"testing"
	"time"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityDocumentValidator_Validate_NilDocument(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	validator := inmemory.NewIdentityDocumentValidator(nil)

	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")

	// Act - Pass nil document
	err := validator.ValidateIdentityDocument(ctx, nil, credential)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "identity document cannot be nil")
}

// TestIdentityDocumentValidator_Validate_NilExpectedID tests nil expected ID error
func TestIdentityDocumentValidator_Validate_NilExpectedID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	validator := inmemory.NewIdentityDocumentValidator(nil)

	// Create a valid document
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
	doc := domain.NewIdentityDocumentFromComponents(
		credential,
		server.GetCA(),
		nil, nil,
		time.Unix(2000000000, 0), // Future expiry
	)

	// Act - Pass nil expected ID
	err = validator.ValidateIdentityDocument(ctx, doc, nil)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected identity credential cannot be nil")
}

// TestIdentityDocumentValidator_Validate_ExpiredDocument tests expired document
func TestIdentityDocumentValidator_Validate_ExpiredDocument(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	validator := inmemory.NewIdentityDocumentValidator(nil)

	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")

	// Create expired document
	doc := domain.NewIdentityDocumentFromComponents(
		credential,
		nil, nil, nil,
		time.Unix(1000000000, 0), // Past expiry
	)

	// Act
	err := validator.ValidateIdentityDocument(ctx, doc, credential)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired or not yet valid")
}

// TestIdentityDocumentValidator_Validate_MismatchedNamespace tests namespace mismatch
func TestIdentityDocumentValidator_Validate_MismatchedNamespace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	validator := inmemory.NewIdentityDocumentValidator(nil)

	td := domain.NewTrustDomainFromName("example.org")
	namespace1 := domain.NewIdentityCredentialFromComponents(td, "/workload1")
	namespace2 := domain.NewIdentityCredentialFromComponents(td, "/workload2")

	// Create document with namespace1
	doc := domain.NewIdentityDocumentFromComponents(
		namespace1,
		nil, nil, nil,
		time.Unix(2000000000, 0), // Future expiry
	)

	// Act - Validate against different namespace2
	err := validator.ValidateIdentityDocument(ctx, doc, namespace2)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected")
	assert.Contains(t, err.Error(), "/workload2")
}

// TestIdentityDocumentValidator_Validate_Success tests successful validation
func TestIdentityDocumentValidator_Validate_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	caCerts := []*x509.Certificate{server.GetCA()}
	bundleProvider := inmemory.NewInMemoryTrustBundleProvider(caCerts)
	validator := inmemory.NewIdentityDocumentValidator(bundleProvider)

	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")

	// Create valid document
	doc := domain.NewIdentityDocumentFromComponents(
		credential,
		nil, nil, nil,
		time.Unix(2000000000, 0), // Future expiry
	)

	// Act
	err = validator.ValidateIdentityDocument(ctx, doc, credential)

	// Assert - Should succeed
	assert.NoError(t, err)
}
