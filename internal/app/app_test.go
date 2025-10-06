package app_test

import (
	"context"
	"testing"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/compose"
	"github.com/pocket/hexagon/spire/internal/adapters/outbound/inmemory"
	"github.com/pocket/hexagon/spire/internal/app"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Placeholder test file - tests can be added here
func TestAppPackage(t *testing.T) {
	t.Parallel()
	// Basic package test
	assert.NotNil(t, app.NewIdentityService)
}

// TestBootstrap_Invariant_SealedRegistry verifies the critical invariant:
// After Bootstrap completes successfully, the registry MUST be sealed to prevent mutations
func TestBootstrap_Invariant_SealedRegistry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Use real implementations to test the actual bootstrap flow
	loader := inmemory.NewInMemoryConfig()
	factory := compose.NewInMemoryAdapterFactory()

	// Bootstrap the application
	application, err := app.Bootstrap(ctx, loader, factory)
	require.NoError(t, err, "Bootstrap should succeed with valid config and factory")
	require.NotNil(t, application, "Application should be non-nil after successful bootstrap")

	// INVARIANT: Registry must be sealed after bootstrap
	// This prevents any mutations to the registry after startup configuration is complete
	_, ok := application.Registry.(*inmemory.InMemoryRegistry)
	require.True(t, ok, "Registry should be InMemoryRegistry type")

	// Test the sealed behavior: attempting to seed a sealed registry should fail
	ctx2 := context.Background()
	testMapper, err := createTestMapper(ctx2, factory)
	require.NoError(t, err)

	// This should fail because registry is sealed
	err = factory.SeedRegistry(application.Registry, ctx2, testMapper)
	assert.Error(t, err, "Invariant violated: Seeded registry must reject further mutations")
	assert.Contains(t, err.Error(), "sealed", "Error should indicate registry is sealed")
}

// Helper function to create a test mapper
func createTestMapper(ctx context.Context, factory *compose.InMemoryAdapterFactory) (*domain.IdentityMapper, error) {
	parser := factory.CreateIdentityNamespaceParser()
	namespace, err := parser.ParseFromString(ctx, "spiffe://test.org/test")
	if err != nil {
		return nil, err
	}

	selector, err := domain.ParseSelectorFromString("unix:uid:9999")
	if err != nil {
		return nil, err
	}

	selectorSet := domain.NewSelectorSet()
	selectorSet.Add(selector)

	return domain.NewIdentityMapper(namespace, selectorSet)
}
