//go:build dev

package inmemory

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
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory/attestor"
	"github.com/pocket/hexagon/spire/internal/domain"
)

func TestAgent_FetchIdentityDocument_NoSelectorsRegistered(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := NewInMemoryTrustDomainParser()
	docProvider := NewInMemoryIdentityDocumentProvider()

	// Create server and registry
	server, err := NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := NewInMemoryRegistry()
	registry.seal()

	// Create attestor with trust domain
	workloadAttestor := attestor.NewUnixPeerCredAttestor("example.org")
	parser := NewInMemoryIdentityCredentialParser()

	agent, err := NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - Try to fetch with current process (will get selectors but no mapper matches)
	workload := domain.NewWorkload(os.Getpid(), os.Getuid(), os.Getgid(), "/testapp")
	_, err = agent.FetchIdentityDocument(ctx, workload)

	// Assert - Should fail because no mapper matches the selectors
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no identity mapper found")
}
func TestAgent_FetchIdentityDocument_NoMatchingMapper(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := NewInMemoryTrustDomainParser()
	docProvider := NewInMemoryIdentityDocumentProvider()

	// Create server and empty registry (no mappers)
	server, err := NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := NewInMemoryRegistry()
	registry.seal() // Seal with no mappers

	// Create attestor with trust domain
	workloadAttestor := attestor.NewUnixPeerCredAttestor("example.org")

	parser := NewInMemoryIdentityCredentialParser()

	agent, err := NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - Fetch with current process (has selectors but no matching mapper in registry)
	workload := domain.NewWorkload(os.Getpid(), os.Getuid(), os.Getgid(), "/testapp")
	_, err = agent.FetchIdentityDocument(ctx, workload)

	// Assert - Should fail with "no mapper found" error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no identity mapper found")
}
func TestAgent_GetIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := NewInMemoryTrustDomainParser()
	docProvider := NewInMemoryIdentityDocumentProvider()

	server, err := NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := NewInMemoryRegistry()
	registry.seal()

	workloadAttestor := attestor.NewUnixPeerCredAttestor("example.org")
	parser := NewInMemoryIdentityCredentialParser()

	agent, err := NewInMemoryAgent(
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
	assert.Equal(t, "spiffe://example.org/agent", identity.IdentityCredential().String())
}
func TestAgent_NewInMemoryAgent_ErrorPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := NewInMemoryTrustDomainParser()
	docProvider := NewInMemoryIdentityDocumentProvider()

	server, err := NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := NewInMemoryRegistry()
	registry.seal()

	workloadAttestor := attestor.NewUnixPeerCredAttestor("example.org")
	parser := NewInMemoryIdentityCredentialParser()

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
			agent, err := NewInMemoryAgent(
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
func TestAgent_FetchIdentityDocument_NoMatchingMapperForSelectors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := NewInMemoryTrustDomainParser()
	docProvider := NewInMemoryIdentityDocumentProvider()

	server, err := NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := NewInMemoryRegistry()
	registry.seal()

	// Create attestor with trust domain
	// Note: UnixPeerCredAttestor generates valid selectors from real process data
	workloadAttestor := attestor.NewUnixPeerCredAttestor("example.org")

	parser := NewInMemoryIdentityCredentialParser()

	agent, err := NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - Try to fetch with current process (has valid selectors but no matching mapper)
	workload := domain.NewWorkload(os.Getpid(), os.Getuid(), os.Getgid(), "/testapp")
	_, err = agent.FetchIdentityDocument(ctx, workload)

	// Assert - Should fail because no mapper matches the selectors
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no identity mapper found")
}
func TestAgent_FetchIdentityDocument_FullErrorFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tdParser := NewInMemoryTrustDomainParser()
	docProvider := NewInMemoryIdentityDocumentProvider()

	server, err := NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := NewInMemoryRegistry()

	// Register a mapper that matches the current process's UID
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/workload")
	// Use current process UID for selector
	currentUID := os.Getuid()
	selectorStr := fmt.Sprintf("unix:uid:%d", currentUID)
	selector, err := domain.ParseSelectorFromString(selectorStr)
	require.NoError(t, err)

	selectorSet := domain.NewSelectorSet()
	selectorSet.Add(selector)
	mapper, err := domain.NewIdentityMapper(credential, selectorSet)
	require.NoError(t, err)
	err = registry.seed(ctx, mapper)
	require.NoError(t, err)
	registry.seal()

	workloadAttestor := attestor.NewUnixPeerCredAttestor("example.org")

	parser := NewInMemoryIdentityCredentialParser()

	agent, err := NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - This should work end-to-end (using current process)
	workload := domain.NewWorkload(os.Getpid(), os.Getuid(), os.Getgid(), "/testapp")
	identity, err := agent.FetchIdentityDocument(ctx, workload)

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
	tdParser := NewInMemoryTrustDomainParser()
	docProvider := NewInMemoryIdentityDocumentProvider()

	server, err := NewInMemoryServer(ctx, "example.org", tdParser, docProvider)
	require.NoError(t, err)

	registry := NewInMemoryRegistry()

	// Register a mapper with root path "/" that matches current process UID
	td := domain.NewTrustDomainFromName("example.org")
	credential := domain.NewIdentityCredentialFromComponents(td, "/")
	currentUID := os.Getuid()
	selectorStr := fmt.Sprintf("unix:uid:%d", currentUID)
	selector, err := domain.ParseSelectorFromString(selectorStr)
	require.NoError(t, err)

	selectorSet := domain.NewSelectorSet()
	selectorSet.Add(selector)
	mapper, err := domain.NewIdentityMapper(credential, selectorSet)
	require.NoError(t, err)
	err = registry.seed(ctx, mapper)
	require.NoError(t, err)
	registry.seal()

	workloadAttestor := attestor.NewUnixPeerCredAttestor("example.org")

	parser := NewInMemoryIdentityCredentialParser()

	agent, err := NewInMemoryAgent(
		ctx,
		"spiffe://example.org/agent",
		server,
		registry,
		workloadAttestor,
		parser,
		docProvider,
	)
	require.NoError(t, err)

	// Act - Fetch with root path identity (using current process)
	workload := domain.NewWorkload(os.Getpid(), os.Getuid(), os.Getgid(), "/testapp")
	identity, err := agent.FetchIdentityDocument(ctx, workload)

	// Assert - Should succeed and return identity document with root path
	require.NoError(t, err)
	assert.NotNil(t, identity)
	// identity is now *domain.IdentityDocument, not *ports.Identity
	assert.NotNil(t, identity.IdentityCredential())
	assert.Equal(t, "spiffe://example.org/", identity.IdentityCredential().String())
}
