package inmemory_test

// Server Coverage Tests
//
// These tests verify edge cases and error paths for the inmemory Server implementation.
// Tests cover server initialization and identity issuance error handling.
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/inmemory/... -v -run TestServer
//	go test ./internal/adapters/outbound/inmemory/... -cover

import (
	"context"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_NewInMemoryServer_ErrorPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	tests := []struct {
		name        string
		trustDomain string
		wantError   bool
	}{
		{"valid trust domain", "example.org", false},
		{"empty trust domain", "", true},
		{"trust domain with scheme", "https://example.org", true},
		{"trust domain with path", "example.org/path", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			server, err := inmemory.NewInMemoryServer(ctx, tt.trustDomain, tdParser, docProvider)

			// Assert
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, server)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, server)
			}
		})
	}
}
func TestServer_NewInMemoryServer_NilDependencies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name      string
		parserNil bool
		provNil   bool
		wantError bool
	}{
		{"nil trust domain parser", true, false, true},
		{"nil cert provider", false, true, true},
		{"both nil", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var parser ports.TrustDomainParser
			var provider ports.IdentityDocumentProvider

			if !tt.parserNil {
				parser = inmemory.NewInMemoryTrustDomainParser()
			}
			if !tt.provNil {
				provider = inmemory.NewInMemoryIdentityDocumentProvider()
			}

			// Act
			server, err := inmemory.NewInMemoryServer(ctx, "example.org", parser, provider)

			// Assert
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, server)
				assert.Contains(t, err.Error(), "trustDomainParser and certProvider are required")
			} else {
				require.NoError(t, err)
				assert.NotNil(t, server)
			}
		})
	}
}

func TestServer_IssueIdentity_NilCredential(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	// Act - Pass nil credential
	doc, err := server.IssueIdentity(ctx, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Contains(t, err.Error(), "identity credential cannot be nil")
}

func TestServer_IssueIdentity_TrustDomainMismatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	idParser := inmemory.NewInMemoryIdentityCredentialParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	// Create server with example.org trust domain
	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	// Create credential with different trust domain
	credential, err := idParser.ParseFromString(ctx, "spiffe://different.org/workload")
	require.NoError(t, err)

	// Act - Try to issue identity for credential with mismatched trust domain
	doc, err := server.IssueIdentity(ctx, credential)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Contains(t, err.Error(), "trust domain mismatch")
	assert.Contains(t, err.Error(), "example.org")
	assert.Contains(t, err.Error(), "different.org")
}
