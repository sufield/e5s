//go:build dev

package inmemory_test

import (
	"context"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer_Invariant_TrustDomainNeverNil tests the invariant:
// "trustDomain is never nil after construction"
func TestServer_Invariant_TrustDomainNeverNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	// Act
	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)

	// Assert invariant
	require.NoError(t, err)
	require.NotNil(t, server)
	assert.NotNil(t, server.GetTrustDomain(),
		"Invariant violated: GetTrustDomain() returned nil")
}

// TestServer_Invariant_CANeverNil tests the invariant:
// "CA certificate and key are never nil after construction"
func TestServer_Invariant_CANeverNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	// Act
	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)

	// Assert invariant
	require.NoError(t, err)
	require.NotNil(t, server)
	assert.NotNil(t, server.GetCA(),
		"Invariant violated: GetCA() returned nil - CA certificate must be initialized")
}

// TestServer_Invariant_IssueIdentityValidatesInput tests the invariant:
// "IssueIdentity() validates inputs before issuing"
func TestServer_Invariant_IssueIdentityValidatesInput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	tests := []struct {
		name       string
		credential *domain.IdentityCredential
		wantError  bool
	}{
		{
			name: "valid credential",
			credential: domain.NewIdentityCredentialFromComponents(
				domain.NewTrustDomainFromName("example.org"), "/workload"),
			wantError: false,
		},
		{
			name:       "nil credential - violates invariant",
			credential: nil,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			doc, err := server.IssueIdentity(ctx, tt.credential)

			// Assert invariant: validates input
			if tt.wantError {
				assert.Error(t, err, "Invariant enforced: should validate input before issuing")
				assert.Nil(t, doc)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, doc)
			}
		})
	}
}

// TestServer_Invariant_IssueIdentityMatchesNamespace tests the invariant:
// "Document's identity credential matches input identityCredential"
func TestServer_Invariant_IssueIdentityMatchesNamespace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	tests := []struct {
		name       string
		credential *domain.IdentityCredential
	}{
		{
			name: "workload credential",
			credential: domain.NewIdentityCredentialFromComponents(
				domain.NewTrustDomainFromName("example.org"), "/workload"),
		},
		{
			name: "service credential",
			credential: domain.NewIdentityCredentialFromComponents(
				domain.NewTrustDomainFromName("example.org"), "/service/api"),
		},
		{
			name: "agent credential",
			credential: domain.NewIdentityCredentialFromComponents(
				domain.NewTrustDomainFromName("example.org"), "/agent"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			doc, err := server.IssueIdentity(ctx, tt.credential)

			// Assert invariant: credential matches
			require.NoError(t, err)
			require.NotNil(t, doc)
			assert.Equal(t, tt.credential, doc.IdentityCredential(),
				"Invariant violated: issued document credential must match input")
		})
	}
}

// TestServer_Invariant_GettersReadOnly tests the invariant:
// "GetTrustDomain() and GetCA() are read-only"
func TestServer_Invariant_GettersReadOnly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	// Get initial values
	initialTD := server.GetTrustDomain()
	initialCA := server.GetCA()

	// Call getters multiple times
	for i := 0; i < 10; i++ {
		_ = server.GetTrustDomain()
		_ = server.GetCA()
	}

	// Assert invariant: values unchanged
	assert.Equal(t, initialTD, server.GetTrustDomain(),
		"Invariant violated: GetTrustDomain() should be read-only")
	assert.Equal(t, initialCA, server.GetCA(),
		"Invariant violated: GetCA() should be read-only")
}

// TestServer_Invariant_IssuedDocumentsAreValid tests the invariant:
// "Freshly issued documents are always valid (non-expired)"
func TestServer_Invariant_IssuedDocumentsAreValid(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	// Arrange
	credential := domain.NewIdentityCredentialFromComponents(
		domain.NewTrustDomainFromName("example.org"), "/workload")

	// Act
	doc, err := server.IssueIdentity(ctx, credential)

	// Assert invariant: freshly issued document is valid
	require.NoError(t, err)
	require.NotNil(t, doc)
	assert.True(t, doc.IsValid(),
		"Invariant violated: freshly issued document should be valid")
	assert.False(t, doc.IsExpired(),
		"Invariant violated: freshly issued document should not be expired")
}
