package inmemory_test

// Agent Coverage Tests
//
// These tests verify edge cases and error paths for the inmemory Agent implementation.
// Tests cover identity fetching, attestation failures, selector matching, and name extraction.
//
// Run these tests with:
//
//	go test ./internal/adapters/outbound/inmemory/... -v -run TestAgent
//	go test ./internal/adapters/outbound/inmemory/... -cover

import (
	"context"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory/attestor"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgent_FetchIdentityDocument_NoSelectorsRegistered(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	// Create server and registry
	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()
	registry.Seal()

	// Create attestor WITHOUT registering any UIDs
	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	parser := inmemory.NewInMemoryIdentityCredentialParser()

	agent, err := inmemory.NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - Try to fetch with unregistered UID
	_, err = agent.FetchIdentityDocument(ctx, ports.ProcessIdentity{UID: 99999, GID: 99999})

	// Assert - Should fail because UID not registered in attestor
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attestation")
}
func TestAgent_FetchIdentityDocument_NoMatchingMapper(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	// Create server and empty registry (no mappers)
	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()
	registry.Seal() // Seal with no mappers

	// Create attestor and register a UID
	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	workloadAttestor.RegisterUID(1000, "unix:user:testuser")

	parser := inmemory.NewInMemoryIdentityCredentialParser()

	agent, err := inmemory.NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - Fetch with UID that has selectors but no matching mapper in registry
	_, err = agent.FetchIdentityDocument(ctx, ports.ProcessIdentity{UID: 1000, GID: 1000})

	// Assert - Should fail with "no mapper found" error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no identity mapper found")
}
func TestAgent_GetIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()
	registry.Seal()

	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	parser := inmemory.NewInMemoryIdentityCredentialParser()

	agent, err := inmemory.NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act
	identity, err := agent.GetIdentity(ctx)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, identity)
	assert.NotNil(t, identity.IdentityCredential)
	assert.Equal(t, "spiffe://example.org/agent", identity.IdentityCredential.String())
}
func TestAgent_NewInMemoryAgent_ErrorPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()
	registry.Seal()

	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	parser := inmemory.NewInMemoryIdentityCredentialParser()

	tests := []struct {
		name        string
		identityURI string
		wantError   bool
	}{
		{"invalid URI", "not-a-spiffe-uri", true},
		{"valid URI", "spiffe://example.org/agent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			agent, err := inmemory.NewInMemoryAgent(
				ctx,
				tt.identityURI,
				server,
				registry,
				workloadAttestor,
				parser,
				docProvider,
			)

			// Assert
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, agent)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, agent)
			}
		})
	}
}
func TestAgent_FetchIdentityDocument_InvalidSelector(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()
	registry.Seal()

	// Create attestor that returns invalid selector format
	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	workloadAttestor.RegisterUID(2000, "invalid-selector-no-colon")

	parser := inmemory.NewInMemoryIdentityCredentialParser()

	agent, err := inmemory.NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - Try to fetch with UID that has invalid selector
	_, err = agent.FetchIdentityDocument(ctx, ports.ProcessIdentity{UID: 2000, GID: 2000})

	// Assert - Should fail with selector parse error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid selector")
}
func TestAgent_FetchIdentityDocument_FullErrorFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()

	// Register a mapper
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
	selector, err := domain.ParseSelectorFromString("unix:uid:1000")
	require.NoError(t, err)

	selectorSet := domain.NewSelectorSet()
	selectorSet.Add(selector)
	mapper, err := domain.NewIdentityMapper(credential, selectorSet)
	require.NoError(t, err)
	err = registry.Seed(ctx, mapper)
	require.NoError(t, err)
	registry.Seal()

	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	workloadAttestor.RegisterUID(1000, "unix:uid:1000")

	parser := inmemory.NewInMemoryIdentityCredentialParser()

	agent, err := inmemory.NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - This should work end-to-end
	identity, err := agent.FetchIdentityDocument(ctx, ports.ProcessIdentity{UID: 1000, GID: 1000})

	// Assert - Should succeed
	require.NoError(t, err)
	assert.NotNil(t, identity)
	// identity is now *domain.IdentityDocument, not *ports.Identity
	// So we check IdentityCredential() method instead of Name and IdentityDocument fields
	assert.NotNil(t, identity.IdentityCredential())
	assert.Equal(t, "spiffe://example.org/workload", identity.IdentityCredential().String())
}

// TestAgent_ExtractName_RootPath tests extractNameFromIdentityCredential with root path
func TestAgent_ExtractName_RootPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := inmemory.NewInMemoryTrustDomainParser()
	docProvider := inmemory.NewInMemoryIdentityDocumentProvider()

	server, err := inmemory.NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := inmemory.NewInMemoryRegistry()

	// Register a mapper with root path "/"
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/")
	selector, err := domain.ParseSelectorFromString("unix:uid:2000")
	require.NoError(t, err)

	selectorSet := domain.NewSelectorSet()
	selectorSet.Add(selector)
	mapper, err := domain.NewIdentityMapper(credential, selectorSet)
	require.NoError(t, err)
	err = registry.Seed(ctx, mapper)
	require.NoError(t, err)
	registry.Seal()

	workloadAttestor := attestor.NewUnixWorkloadAttestor()
	workloadAttestor.RegisterUID(2000, "unix:uid:2000")

	parser := inmemory.NewInMemoryIdentityCredentialParser()

	agent, err := inmemory.NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - Fetch with root path identity
	identity, err := agent.FetchIdentityDocument(ctx, ports.ProcessIdentity{UID: 2000, GID: 2000})

	// Assert - Should succeed and return identity document with root path
	require.NoError(t, err)
	assert.NotNil(t, identity)
	// identity is now *domain.IdentityDocument, not *ports.Identity
	assert.NotNil(t, identity.IdentityCredential())
	assert.Equal(t, "spiffe://example.org/", identity.IdentityCredential().String())
}
